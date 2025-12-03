package cmd

import (
	"fmt"
	"go/importer"
	"go/printer"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/amirkhaki/moriarty/pkg/instrument"
	"github.com/spf13/cobra"
)

// toolexecCmd represents the toolexec command
var toolexecCmd = &cobra.Command{
	Use:                "toolexec",
	Short:              "go build -toolexec 'moriarty toolexec'",
	Long:               ``,
	DisableFlagParsing: true,
	Args:               cobra.MinimumNArgs(1),
	Run:                handleToolExec,
}

// handleToolExec intercepts go tool commands when used with -toolexec
func handleToolExec(cmd *cobra.Command, args []string) {
	// Args: [moriarty, /path/to/compile, compile-args...]
	tool := args[0]
	args = args[1:]

	// Handle link command separately
	if strings.HasSuffix(tool, "link") {
		handleLinkCommand(tool, args)
		return
	}

	// Only instrument for compile commands
	if !strings.HasSuffix(tool, "compile") {
		// Pass through for other tools (asm, etc.)
		cmd := exec.Command(tool, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				os.Exit(exitErr.ExitCode())
			}
			os.Exit(1)
		}
		return
	}

	// Find .go source files and importcfg in arguments
	var goFiles []string
	var newArgs []string
	var importcfgPath string
	tempDir := ""

	// Get GOROOT to filter out standard library files
	goroot := os.Getenv("GOROOT")
	if goroot == "" {
		// Get it from go env if not set
		cmd := exec.Command("go", "env", "GOROOT")
		if out, err := cmd.Output(); err == nil {
			goroot = strings.TrimSpace(string(out))
		}
	}

	for i, arg := range args {
		if strings.HasSuffix(arg, ".go") && !strings.HasPrefix(arg, "-") {
			// Skip files in GOROOT
			if goroot != "" && strings.HasPrefix(filepath.Clean(arg), filepath.Clean(goroot)) {
				continue
			}
			goFiles = append(goFiles, arg)
		}
		if arg == "-importcfg" && i+1 < len(args) {
			importcfgPath = args[i+1]
		}
	}

	// If no .go files, just pass through
	if len(goFiles) == 0 {
		cmd := exec.Command(tool, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				os.Exit(exitErr.ExitCode())
			}
			os.Exit(1)
		}
		return
	}

	// Use Go's work directory if available, otherwise create temp directory
	var err error
	tempDir = os.Getenv("WORK")
	if tempDir == "" {
		tempDir, err = os.MkdirTemp("", "moriarty_*")
		if err != nil {
			fmt.Fprintf(os.Stderr, "moriarty: failed to create temp dir: %v\n", err)
			os.Exit(1)
		}
		defer os.RemoveAll(tempDir)
	}

	// Instrument all .go files together (for proper type checking)
	var customImporter types.Importer
	if importcfgPath != "" {
		// Create importer from importcfg
		var err error
		customImporter, err = createImporterFromCfg(importcfgPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "moriarty: warning: failed to create importer from cfg: %v\n", err)
			// Fall back to default importer
			customImporter = nil
		}
	}

	instrumentedFiles, wasInstrumented, err := instrumentFilesToDir(goFiles, tempDir, customImporter)
	if err != nil {
		fmt.Fprintf(os.Stderr, "moriarty: failed to instrument: %v\n", err)
		os.Exit(1)
	}

	// Build map of original -> instrumented file paths
	fileMap := make(map[string]string)
	for i, origFile := range goFiles {
		fileMap[origFile] = instrumentedFiles[i]
	}

	// Only modify importcfg if we actually added instrumentation
	newImportcfgPath := importcfgPath
	if wasInstrumented && importcfgPath != "" {
		newImportcfgPath, err = modifyImportCfg(importcfgPath, tempDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "moriarty: failed to modify importcfg: %v\n", err)
			os.Exit(1)
		}
	}

	// Replace original files with instrumented versions and update importcfg in args
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if instrumented, ok := fileMap[arg]; ok {
			newArgs = append(newArgs, instrumented)
		} else if arg == "-importcfg" && newImportcfgPath != importcfgPath {
			newArgs = append(newArgs, arg)
			i++ // Move to next arg
			if i < len(args) {
				newArgs = append(newArgs, newImportcfgPath) // Use modified importcfg
			}
		} else {
			newArgs = append(newArgs, arg)
		}
	}

	// Run the original compile command with instrumented files
	command := exec.Command(tool, newArgs...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.Stdin = os.Stdin
	err = command.Run()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}
}

// createImporterFromCfg creates a types.Importer from an importcfg file
func createImporterFromCfg(importcfgPath string) (types.Importer, error) {
	content, err := os.ReadFile(importcfgPath)
	if err != nil {
		return nil, err
	}

	// Parse importcfg to build package map
	packageMap := make(map[string]string)
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "packagefile ") {
			// Format: packagefile path=archive
			parts := strings.SplitN(line[12:], "=", 2)
			if len(parts) == 2 {
				packageMap[parts[0]] = parts[1]
			}
		}
	}

	// Create an importer using the package map
	// Use ForCompiler with gcexportdata for .a files
	defaultImporter := importer.Default()
	return &importCfgImporter{
		packageMap:      packageMap,
		defaultImporter: defaultImporter,
	}, nil
}

// importCfgImporter implements types.Importer using an importcfg package map
type importCfgImporter struct {
	packageMap      map[string]string
	defaultImporter types.Importer
}

func (imp *importCfgImporter) Import(path string) (*types.Package, error) {
	// Try to find package in our map
	if archivePath, ok := imp.packageMap[path]; ok {
		// Use ForCompiler to read .a files
		gcImporter := importer.ForCompiler(token.NewFileSet(), "gc", func(p string) (io.ReadCloser, error) {
			return os.Open(archivePath)
		})
		return gcImporter.Import(path)
	}

	// Fall back to default importer
	return imp.defaultImporter.Import(path)
}

// modifyImportCfg adds our runtime package to the importcfg file
func modifyImportCfg(originalPath, tempDir string) (string, error) {
	// Read original importcfg
	content, err := os.ReadFile(originalPath)
	if err != nil {
		return "", err
	}

	// Compile the runtime package directly
	runtimePkgPath := "github.com/amirkhaki/moriarty/pkg/runtime"
	archivePath := filepath.Join(tempDir, "runtime.a")

	// Find moriarty project root (where this binary is from)
	// Assume it's in bin/ subdirectory
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}
	projectRoot := filepath.Dir(filepath.Dir(exePath))
	runtimeSrcDir := filepath.Join(projectRoot, "pkg", "runtime")

	// Get go tool compile path
	compileCmd := exec.Command("go", "env", "GOTOOLDIR")
	toolDir, err := compileCmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get GOTOOLDIR: %w", err)
	}
	compilePath := filepath.Join(strings.TrimSpace(string(toolDir)), "compile")

	// Compile runtime.go to archive
	runtimeSrc := filepath.Join(runtimeSrcDir, "runtime.go")
	cmd := exec.Command(compilePath, "-o", archivePath, "-p", runtimePkgPath, runtimeSrc)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("failed to compile runtime package: %w\nOutput: %s", err, string(output))
	}

	// Create new importcfg with our package added
	newContent := string(content)
	runtimeEntry := fmt.Sprintf("packagefile %s=%s\n", runtimePkgPath, archivePath)

	// Add it at the end
	newContent = newContent + runtimeEntry

	// Write modified importcfg
	newPath := filepath.Join(tempDir, "importcfg")
	if err := os.WriteFile(newPath, []byte(newContent), 0644); err != nil {
		return "", err
	}

	return newPath, nil
}

// modifyLinkImportCfg adds our runtime package to the link importcfg file
func modifyLinkImportCfg(originalPath, tempDir string) (string, error) {
	// Read original importcfg
	content, err := os.ReadFile(originalPath)
	if err != nil {
		return "", err
	}

	// Find the runtime.a file we compiled earlier
	runtimePkgPath := "github.com/amirkhaki/moriarty/pkg/runtime"
	archivePath := filepath.Join(tempDir, "runtime.a")

	// Check if it exists (it should have been created during compile step)
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		// Compile it now if it doesn't exist
		exePath, err := os.Executable()
		if err != nil {
			return "", fmt.Errorf("failed to get executable path: %w", err)
		}
		projectRoot := filepath.Dir(filepath.Dir(exePath))
		runtimeSrcDir := filepath.Join(projectRoot, "pkg", "runtime")

		// Get go tool compile path
		compileCmd := exec.Command("go", "env", "GOTOOLDIR")
		toolDir, err := compileCmd.Output()
		if err != nil {
			return "", fmt.Errorf("failed to get GOTOOLDIR: %w", err)
		}
		compilePath := filepath.Join(strings.TrimSpace(string(toolDir)), "compile")

		// Compile runtime.go to archive
		runtimeSrc := filepath.Join(runtimeSrcDir, "runtime.go")
		cmd := exec.Command(compilePath, "-o", archivePath, "-p", runtimePkgPath, runtimeSrc)
		if output, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("failed to compile runtime package: %w\nOutput: %s", err, string(output))
		}
	}

	// Create new importcfg with our package added
	newContent := string(content)
	runtimeEntry := fmt.Sprintf("packagefile %s=%s\n", runtimePkgPath, archivePath)

	// Add it at the end
	newContent = newContent + runtimeEntry

	// Write modified importcfg
	newPath := filepath.Join(tempDir, "importcfg.link")
	if err := os.WriteFile(newPath, []byte(newContent), 0644); err != nil {
		return "", err
	}

	return newPath, nil
}

// instrumentFilesToDir instruments multiple files together and writes them to the target directory
// Returns the instrumented file paths and whether any instrumentation was added
func instrumentFilesToDir(goFiles []string, targetDir string, customImporter types.Importer) ([]string, bool, error) {
	cfg := instrument.DefaultConfig()
	cfg.Importer = customImporter
	instr := instrument.NewInstrumenter(cfg)
	fset := token.NewFileSet()

	// Instrument all files together (for proper type checking across files)
	instrumentedASTs, err := instr.InstrumentFiles(fset, goFiles)
	if err != nil {
		return nil, false, err
	}

	// Write each instrumented file to the target directory
	outputFiles := make([]string, len(goFiles))
	for i, origFile := range goFiles {
		baseName := filepath.Base(origFile)
		outputPath := filepath.Join(targetDir, baseName)

		f, err := os.Create(outputPath)
		if err != nil {
			return nil, false, fmt.Errorf("failed to create %s: %w", outputPath, err)
		}

		err = printer.Fprint(f, fset, instrumentedASTs[i])
		f.Close()
		if err != nil {
			return nil, false, fmt.Errorf("failed to write %s: %w", outputPath, err)
		}

		outputFiles[i] = outputPath
	}

	return outputFiles, instr.WasInstrumented(), nil
}
func init() {
	rootCmd.AddCommand(toolexecCmd)
}

// handleLinkCommand intercepts link commands and adds our runtime package to importcfg
func handleLinkCommand(tool string, args []string) {
	// Find importcfg in arguments
	var importcfgPath string
	for i, arg := range args {
		if arg == "-importcfg" && i+1 < len(args) {
			// Expand environment variables in the path
			importcfgPath = os.ExpandEnv(args[i+1])
			break
		}
	}

	if importcfgPath == "" {
		// No importcfg, just run the link command
		cmd := exec.Command(tool, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		err := cmd.Run()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				os.Exit(exitErr.ExitCode())
			}
			os.Exit(1)
		}
		return
	}

	// Create a temp directory for our runtime package
	tempDir, err := os.MkdirTemp("", "moriarty_link_*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "moriarty: warning: failed to create temp dir: %v\n", err)
		// Continue without modification
		cmd := exec.Command(tool, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		err := cmd.Run()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				os.Exit(exitErr.ExitCode())
			}
			os.Exit(1)
		}
		return
	}
	defer os.RemoveAll(tempDir)

	// Modify the importcfg to include our runtime package
	newImportcfgPath, err := modifyLinkImportCfg(importcfgPath, tempDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "moriarty: warning: failed to modify link importcfg: %v\n", err)
	} else {
		// Replace the importcfg path in args
		for i, arg := range args {
			if arg == "-importcfg" && i+1 < len(args) {
				args[i+1] = newImportcfgPath
				break
			}
		}
	}

	// Run the link command
	cmd := exec.Command(tool, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err = cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}
}
