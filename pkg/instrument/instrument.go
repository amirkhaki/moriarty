package instrument

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"golang.org/x/tools/go/ast/astutil"
	"io"
)

// Config holds configuration for the instrumentation
type Config struct {
	// ImportRewrites maps import paths to replacement paths
	ImportRewrites map[string]string

	// BaseRuntimeAddress is the base package path for runtime functions
	BaseRuntimeAddress string

	// RuntimeAlias is the import alias for the runtime package
	// If empty, a mangled name will be generated from BaseRuntimeAddress
	RuntimeAlias string

	// MemReadFunc is the name of the memory read function
	MemReadFunc string

	// MemWriteFunc is the name of the memory write function
	MemWriteFunc string

	// SpawnFunc is the name of the goroutine spawn function
	SpawnFunc string

	// GoroutineEnterFunc is the name of the goroutine enter hook
	GoroutineEnterFunc string

	// GoroutineExitFunc is the name of the goroutine exit hook
	GoroutineExitFunc string

	// Importer is used for resolving imports during type checking
	// If nil, importer.Default() is used
	Importer types.Importer
}

// DefaultConfig returns a Config with default settings
func DefaultConfig() *Config {
	baseAddr := "github.com/amirkhaki/moriarty/pkg/runtime"
	return &Config{
		BaseRuntimeAddress: baseAddr,
		RuntimeAlias:       "", // Will be auto-generated
		MemReadFunc:        "MemRead",
		MemWriteFunc:       "MemWrite",
		SpawnFunc:          "Spawn",
		GoroutineEnterFunc: "GoroutineEnter",
		GoroutineExitFunc:  "GoroutineExit",
		ImportRewrites:     map[string]string{},
	}
}

// Instrumenter handles the instrumentation of Go source code
type Instrumenter struct {
	config          *Config
	typeInfo        *types.Info
	instrumented    bool // tracks if any instrumentation was added to current file
	anyInstrumented bool // tracks if any file had instrumentation
}

// NewInstrumenter creates a new Instrumenter with the given config
func NewInstrumenter(config *Config) *Instrumenter {
	if config == nil {
		config = DefaultConfig()
	}

	// Generate runtime alias if not provided
	if config.RuntimeAlias == "" {
		config.RuntimeAlias = generateRuntimeAlias(config.BaseRuntimeAddress)
	}

	return &Instrumenter{
		config: config,
	}
}

// generateRuntimeAlias creates a deterministic mangled alias from the import path
// This ensures no conflicts with user imports
func generateRuntimeAlias(importPath string) string {
	// Use SHA256 hash for deterministic mangling
	hash := sha256.Sum256([]byte(importPath))
	// Take first 8 bytes and hex encode for a 16-char suffix
	hashStr := hex.EncodeToString(hash[:8])
	// Create alias: __moriarty_<hash>
	return "__moriarty_" + hashStr
}

// WasInstrumented returns true if any instrumentation was added during the last operation
func (instr *Instrumenter) WasInstrumented() bool {
	return instr.anyInstrumented
}

// InstrumentFile instruments a single Go source file
func (instr *Instrumenter) InstrumentFile(fset *token.FileSet, filename string, src interface{}) (*ast.File, error) {
	// Parse the file
	f, err := parser.ParseFile(fset, filename, src, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	return instr.InstrumentAST(fset, f)
}

// InstrumentFiles instruments multiple Go source files together (for proper type checking)
func (instr *Instrumenter) InstrumentFiles(fset *token.FileSet, filenames []string) ([]*ast.File, error) {
	// Parse all files
	files := make([]*ast.File, len(filenames))
	for i, filename := range filenames {
		f, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", filename, err)
		}
		files[i] = f
	}

	return instr.InstrumentASTs(fset, files)
}

// InstrumentASTs instruments multiple already-parsed ASTs together
func (instr *Instrumenter) InstrumentASTs(fset *token.FileSet, files []*ast.File) ([]*ast.File, error) {
	// Reset the any-instrumented flag for this batch
	instr.anyInstrumented = false

	// Perform type checking on all files together
	imp := instr.config.Importer
	if imp == nil {
		imp = importer.Default()
	}
	conf := types.Config{Importer: imp}
	instr.typeInfo = &types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
		Defs:  make(map[*ast.Ident]types.Object),
		Uses:  make(map[*ast.Ident]types.Object),
	}
	_, typeErr := conf.Check("", fset, files, instr.typeInfo)
	// If type checking completely failed (no useful type info), disable it
	if typeErr != nil && len(instr.typeInfo.Defs) == 0 && len(instr.typeInfo.Uses) == 0 {
		instr.typeInfo = nil
	}
	// Otherwise, we can use partial type info even if there were errors

	// Instrument each file
	for _, f := range files {
		instr.instrumentSingleAST(fset, f)
	}

	return files, nil
}

// InstrumentAST instruments an already-parsed AST
func (instr *Instrumenter) InstrumentAST(fset *token.FileSet, f *ast.File) (*ast.File, error) {
	// Perform type checking on single file
	imp := instr.config.Importer
	if imp == nil {
		imp = importer.Default()
	}
	conf := types.Config{Importer: imp}
	instr.typeInfo = &types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
		Defs:  make(map[*ast.Ident]types.Object),
		Uses:  make(map[*ast.Ident]types.Object),
	}
	_, typeErr := conf.Check("", fset, []*ast.File{f}, instr.typeInfo)
	// If type checking completely failed (no useful type info), disable it
	if typeErr != nil && len(instr.typeInfo.Defs) == 0 && len(instr.typeInfo.Uses) == 0 {
		instr.typeInfo = nil
	}
	// Otherwise, we can use partial type info even if there were errors

	instr.instrumentSingleAST(fset, f)
	return f, nil
}

// instrumentSingleAST performs the actual instrumentation on a single file
// (assumes typeInfo is already populated)
func (instr *Instrumenter) instrumentSingleAST(fset *token.FileSet, f *ast.File) {

	// Apply import rewrites
	for k, v := range instr.config.ImportRewrites {
		astutil.RewriteImport(fset, f, k, v)
	}

	// Reset instrumentation flag
	instr.instrumented = false

	// Pass 0: Lower control flow structures (if/for with init)
	astutil.Apply(f, nil, func(c *astutil.Cursor) bool {
		switch n := c.Node().(type) {
		case *ast.IfStmt:
			instr.lowerIfStmt(c, n)
		case *ast.ForStmt:
			instr.lowerForStmt(c, n)
		}
		return true
	})

	// Apply instrumentation (first pass: everything except go statements)
	astutil.Apply(f, nil, func(c *astutil.Cursor) bool {
		switch n := c.Node().(type) {
		case *ast.IfStmt:
			instr.instrumentIfStmt(c, n)
		case *ast.ForStmt:
			instr.instrumentForStmt(c, n)
		case *ast.SwitchStmt:
			instr.instrumentSwitchStmt(c, n)
		case *ast.IncDecStmt:
			instr.instrumentIncDec(c, n)
		case *ast.AssignStmt:
			instr.instrumentAssignment(c, n)
		case *ast.SendStmt:
			instr.instrumentSend(c, n)
		case *ast.RangeStmt:
			instr.instrumentRange(c, n)
		case *ast.ReturnStmt:
			instr.instrumentReturn(c, n)
		case *ast.ExprStmt:
			instr.instrumentExprStmt(c, n)
		}
		return true
	})

	// Second pass: instrument go statements after all other instrumentation is done
	astutil.Apply(f, nil, func(c *astutil.Cursor) bool {
		if stmt, ok := c.Node().(*ast.GoStmt); ok {
			instr.instrumentGoStmt(c, stmt)
		}
		return true
	})

	// Third pass: instrument main function if this is the main package
	instr.instrumentMainFunction(f)

	// Only add imports if instrumentation was actually added
	if instr.instrumented {
		instr.anyInstrumented = true
		astutil.AddImport(fset, f, "unsafe")
		astutil.AddNamedImport(fset, f, instr.config.RuntimeAlias, instr.config.BaseRuntimeAddress)
	}
}

// WriteInstrumented writes the instrumented AST to the given writer
func WriteInstrumented(w io.Writer, fset *token.FileSet, f *ast.File) error {
	return ast.Fprint(w, fset, f, nil)
}

func (instr *Instrumenter) makeMemReadCall(expr ast.Expr) *ast.CallExpr {
	instr.instrumented = true
	return &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   &ast.Ident{Name: instr.config.RuntimeAlias},
			Sel: &ast.Ident{Name: instr.config.MemReadFunc},
		},
		Args: []ast.Expr{
			&ast.CallExpr{
				Fun: &ast.SelectorExpr{
					X:   &ast.Ident{Name: "unsafe"},
					Sel: &ast.Ident{Name: "Pointer"},
				},
				Args: []ast.Expr{
					&ast.UnaryExpr{Op: token.AND, X: expr},
				},
			},
		},
	}
}

func (instr *Instrumenter) makeMemWriteCall(expr ast.Expr) *ast.CallExpr {
	instr.instrumented = true
	return &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   &ast.Ident{Name: instr.config.RuntimeAlias},
			Sel: &ast.Ident{Name: instr.config.MemWriteFunc},
		},
		Args: []ast.Expr{
			&ast.CallExpr{
				Fun: &ast.SelectorExpr{
					X:   &ast.Ident{Name: "unsafe"},
					Sel: &ast.Ident{Name: "Pointer"},
				},
				Args: []ast.Expr{
					&ast.UnaryExpr{Op: token.AND, X: expr},
				},
			},
		},
	}
}

func (instr *Instrumenter) instrumentGoStmt(c *astutil.Cursor, stmt *ast.GoStmt) {
	// Transform: go f(expr1, expr2, ...)
	// Into: {
	//   MemRead(expr1)  // if expr1 is a variable
	//   MemRead(expr2)  // if expr2 is a variable
	//   p1 := expr1
	//   p2 := expr2
	//   ...
	//   runtime.Spawn(func() {
	//     runtime.GoroutineEnter()
	//     f(p1, p2, ...)
	//     runtime.GoroutineExit()
	//   })
	// }

	instr.instrumented = true

	callExpr := stmt.Call

	var blockStmts []ast.Stmt
	var paramIdents []ast.Expr

	// Create temporary variables for each argument to evaluate them before spawning
	for i, arg := range callExpr.Args {
		// Add memory read instrumentation for the argument
		instr.collectReads(arg, &blockStmts)

		// Generate unique parameter name
		paramName := &ast.Ident{Name: fmt.Sprintf("__moriarty_p%d", i)}

		// Create assignment: pN := argN
		assignStmt := &ast.AssignStmt{
			Lhs: []ast.Expr{paramName},
			Tok: token.DEFINE,
			Rhs: []ast.Expr{arg},
		}
		blockStmts = append(blockStmts, assignStmt)
		paramIdents = append(paramIdents, paramName)
	}

	// Create the wrapped function call with the parameter identifiers
	wrappedCall := &ast.CallExpr{
		Fun:  callExpr.Fun,
		Args: paramIdents,
	}

	// Create runtime.GoroutineEnter() call
	enterCall := &ast.ExprStmt{
		X: &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   &ast.Ident{Name: instr.config.RuntimeAlias},
				Sel: &ast.Ident{Name: instr.config.GoroutineEnterFunc},
			},
		},
	}

	// Create runtime.GoroutineExit() call
	exitCall := &ast.ExprStmt{
		X: &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   &ast.Ident{Name: instr.config.RuntimeAlias},
				Sel: &ast.Ident{Name: instr.config.GoroutineExitFunc},
			},
		},
	}

	// Create the function literal: func() { GoroutineEnter(); f(p1, p2, ...); GoroutineExit() }
	funcLit := &ast.FuncLit{
		Type: &ast.FuncType{Params: &ast.FieldList{}},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				enterCall,
				&ast.ExprStmt{X: wrappedCall},
				exitCall,
			},
		},
	}

	// Create runtime.Spawn(funcLit) call
	spawnCall := &ast.ExprStmt{
		X: &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   &ast.Ident{Name: instr.config.RuntimeAlias},
				Sel: &ast.Ident{Name: instr.config.SpawnFunc},
			},
			Args: []ast.Expr{funcLit},
		},
	}
	blockStmts = append(blockStmts, spawnCall)

	// Replace the go statement with a block statement
	blockStmt := &ast.BlockStmt{List: blockStmts}
	c.Replace(blockStmt)
}

// lowerIfStmt transforms: if init; cond { body }
// Into: { init; if cond { body } }
func (instr *Instrumenter) lowerIfStmt(c *astutil.Cursor, stmt *ast.IfStmt) {
	if stmt.Init != nil && canInsertBefore(c) {
		// Create a block containing init and the if statement
		block := &ast.BlockStmt{
			List: []ast.Stmt{
				stmt.Init,
				stmt,
			},
		}
		stmt.Init = nil
		c.Replace(block)
	}
}

// lowerForStmt transforms: for init; cond; post { body }
// Into: { init; for cond { body; post } }
func (instr *Instrumenter) lowerForStmt(c *astutil.Cursor, stmt *ast.ForStmt) {
	if !canInsertBefore(c) {
		return // Can't lower if we can't insert before
	}

	hasInit := stmt.Init != nil
	hasPost := stmt.Post != nil

	// Move post to end of body if present
	if hasPost && stmt.Body != nil {
		stmt.Body.List = append(stmt.Body.List, stmt.Post)
		stmt.Post = nil
	}

	// Wrap in block if we have init
	if hasInit {
		block := &ast.BlockStmt{
			List: []ast.Stmt{
				stmt.Init,
				stmt,
			},
		}
		stmt.Init = nil
		c.Replace(block)
	}
}

func (instr *Instrumenter) instrumentIfStmt(c *astutil.Cursor, stmt *ast.IfStmt) {
	// After lowering, just instrument the condition
	if stmt.Cond != nil && canInsertBefore(c) {
		var readStmts []ast.Stmt
		instr.collectReads(stmt.Cond, &readStmts)
		for _, s := range readStmts {
			c.InsertBefore(s)
		}
	}
}

func (instr *Instrumenter) instrumentForStmt(c *astutil.Cursor, stmt *ast.ForStmt) {
	// After lowering, condition is evaluated BEFORE loop and BEFORE each iteration
	if stmt.Cond != nil {
		var readStmts []ast.Stmt
		instr.collectReads(stmt.Cond, &readStmts)

		if canInsertBefore(c) {
			// Add reads before the loop (for first condition check)
			for _, s := range readStmts {
				c.InsertBefore(s)
			}
		}

		// Also append to body (AFTER body and post, BEFORE next iteration's condition check)
		if stmt.Body != nil && len(readStmts) > 0 {
			stmt.Body.List = append(stmt.Body.List, readStmts...)
		}
	}
}

func (instr *Instrumenter) instrumentSwitchStmt(c *astutil.Cursor, stmt *ast.SwitchStmt) {
	// Instrument the tag expression
	if stmt.Tag != nil && canInsertBefore(c) {
		var readStmts []ast.Stmt
		instr.collectReads(stmt.Tag, &readStmts)

		// Insert reads BEFORE the switch statement
		for _, s := range readStmts {
			c.InsertBefore(s)
		}
	}
}

func (instr *Instrumenter) instrumentIncDec(c *astutil.Cursor, stmt *ast.IncDecStmt) {
	// Check if we can insert before - skip if in for loop init/post or other non-list contexts
	if !canInsertBefore(c) {
		return
	}
	memReadCall := &ast.ExprStmt{X: instr.makeMemReadCall(stmt.X)}
	memWriteCall := &ast.ExprStmt{X: instr.makeMemWriteCall(stmt.X)}
	c.InsertBefore(memReadCall)
	c.InsertBefore(memWriteCall)
}

// canInsertBefore checks if the cursor is in a context where InsertBefore will work.
// InsertBefore only works when the current node is in a slice field of its parent.
func canInsertBefore(c *astutil.Cursor) bool {
	// Index() returns >= 0 if the node is in a slice, < 0 otherwise
	return c.Index() >= 0
}

func isBlankIdent(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == "_"
}

func (instr *Instrumenter) instrumentAssignment(c *astutil.Cursor, stmt *ast.AssignStmt) {
	if !canInsertBefore(c) {
		return
	}

	var readStmts, writeStmts []ast.Stmt

	// For regular assignment and op-assign, RHS values are read
	for _, rhs := range stmt.Rhs {
		instr.collectReads(rhs, &readStmts)
	}

	// For LHS: handle based on assignment type
	for _, lhs := range stmt.Lhs {
		if isBlankIdent(lhs) {
			continue
		}

		// For op-assignments (+=, -=, etc.), LHS is also read
		if stmt.Tok != token.ASSIGN && stmt.Tok != token.DEFINE {
			instr.collectReads(lhs, &readStmts)
		}

		// For := we need to check if the variable is being redeclared in the same scope
		// vs being defined for the first time
		if stmt.Tok == token.DEFINE {
			if ident, ok := lhs.(*ast.Ident); ok {
				// Only instrument if we have type info AND it's a redeclaration
				// Defs[ident] == nil means the variable was NOT newly defined here (redeclaration)
				// Defs[ident] != nil means this is the first definition (new variable)
				if instr.typeInfo != nil && instr.typeInfo.Defs[ident] == nil {
					// This is a redeclaration - instrument the write
					instr.collectWrites(lhs, &writeStmts)
				}
				// Otherwise it's a new variable or we can't tell - no instrumentation
			} else {
				// LHS is not a simple identifier (e.g., a.b := ...), instrument it
				instr.collectWrites(lhs, &writeStmts)
			}
		} else {
			// Regular assignment or op-assignment - always instrument
			instr.collectWrites(lhs, &writeStmts)
		}
	}

	// Insert all instrumentation BEFORE the statement
	for _, s := range readStmts {
		c.InsertBefore(s)
	}
	for _, s := range writeStmts {
		c.InsertBefore(s)
	}
}

func (instr *Instrumenter) instrumentSend(c *astutil.Cursor, stmt *ast.SendStmt) {
	if !canInsertBefore(c) {
		return
	}
	var readStmts []ast.Stmt
	instr.collectReads(stmt.Chan, &readStmts)
	instr.collectReads(stmt.Value, &readStmts)
	for _, s := range readStmts {
		c.InsertBefore(s)
	}
}

func (instr *Instrumenter) instrumentRange(c *astutil.Cursor, stmt *ast.RangeStmt) {
	if !canInsertBefore(c) {
		return
	}

	var readStmts, writeStmts []ast.Stmt

	// Range expression is read
	instr.collectReads(stmt.X, &readStmts)

	// Collect writes for key and value
	if stmt.Key != nil && !isBlankIdent(stmt.Key) {
		instr.collectWrites(stmt.Key, &writeStmts)
	}
	if stmt.Value != nil && !isBlankIdent(stmt.Value) {
		instr.collectWrites(stmt.Value, &writeStmts)
	}

	// Insert reads before
	for _, s := range readStmts {
		c.InsertBefore(s)
	}

	// For range with :=, writes happen after (inside loop body conceptually)
	// For range with =, writes happen before
	if stmt.Tok == token.DEFINE {
		// For :=, we need to insert instrumentation at the beginning of the loop body
		if stmt.Body != nil && len(writeStmts) > 0 {
			// Prepend write instrumentation to the loop body
			stmt.Body.List = append(writeStmts, stmt.Body.List...)
		}
	} else {
		for _, s := range writeStmts {
			c.InsertBefore(s)
		}
	}
}

func (instr *Instrumenter) instrumentReturn(c *astutil.Cursor, stmt *ast.ReturnStmt) {
	if !canInsertBefore(c) {
		return
	}
	var readStmts []ast.Stmt
	for _, result := range stmt.Results {
		instr.collectReads(result, &readStmts)
	}
	for _, s := range readStmts {
		c.InsertBefore(s)
	}
}

func (instr *Instrumenter) instrumentExprStmt(c *astutil.Cursor, stmt *ast.ExprStmt) {
	if !canInsertBefore(c) {
		return
	}
	// Instrument reads in expression statements (e.g., function calls with variable arguments)
	var readStmts []ast.Stmt
	instr.collectReads(stmt.X, &readStmts)
	for _, s := range readStmts {
		c.InsertBefore(s)
	}
}

func (instr *Instrumenter) collectReads(expr ast.Expr, stmts *[]ast.Stmt) {
	if expr == nil {
		return
	}

	switch e := expr.(type) {
	case *ast.Ident:
		if isBuiltin(e.Name) {
			return
		}
		// Skip package identifiers and type names
		if instr.typeInfo != nil {
			if obj := instr.typeInfo.Uses[e]; obj != nil {
				// Skip if it's a package name, type name, constant, nil, or function
				switch obj.(type) {
				case *types.PkgName, *types.TypeName, *types.Const, *types.Nil, *types.Func:
					return
				}
			}
		}
		*stmts = append(*stmts, &ast.ExprStmt{X: instr.makeMemReadCall(e)})
	case *ast.SelectorExpr:
		// Check if the selector is a constant before instrumenting
		if instr.typeInfo != nil {
			if obj := instr.typeInfo.Uses[e.Sel]; obj != nil {
				// Skip if it's a constant, package name, or type name
				switch obj.(type) {
				case *types.Const, *types.PkgName, *types.TypeName, *types.Nil:
					return
				}
			} else {
				// No object found for selector - check if X is an identifier (likely package)
				if ident, ok := e.X.(*ast.Ident); ok {
					if xObj := instr.typeInfo.Uses[ident]; xObj != nil {
						if _, isPkg := xObj.(*types.PkgName); isPkg {
							return
						}
					} else {
						// X not in Uses either - assume it's a package
						return
					}
				}
			}
		} else {
			// No type info - be conservative
			// If X is a simple identifier, it's likely a package name, skip it
			if _, isIdent := e.X.(*ast.Ident); isIdent {
				return
			}
		}
		instr.collectReads(e.X, stmts)
		*stmts = append(*stmts, &ast.ExprStmt{X: instr.makeMemReadCall(e)})
	case *ast.IndexExpr:
		// Use type information to determine if this is a map or array/slice
		instr.collectReads(e.X, stmts)
		instr.collectReads(e.Index, stmts)

		// Check if the indexed expression is addressable (not a map)
		if instr.typeInfo != nil {
			if tv, ok := instr.typeInfo.Types[e.X]; ok {
				// If it's a map, we can't take address of the element
				if _, isMap := tv.Type.Underlying().(*types.Map); !isMap {
					// It's an array or slice, we can read the element
					*stmts = append(*stmts, &ast.ExprStmt{X: instr.makeMemReadCall(e)})
				}
			}
		}
	case *ast.StarExpr:
		instr.collectReads(e.X, stmts)
		*stmts = append(*stmts, &ast.ExprStmt{X: instr.makeMemReadCall(e)})
	case *ast.UnaryExpr:
		if e.Op == token.AND {
			// Taking address (&x) doesn't read the value, skip instrumentation
			return
		} else if e.Op == token.ARROW {
			// Channel receive (<-ch) reads from the channel
			instr.collectReads(e.X, stmts)
		} else {
			// Other unary ops (!, -, +, ^) read the operand
			instr.collectReads(e.X, stmts)
		}
	case *ast.BinaryExpr:
		instr.collectReads(e.X, stmts)
		instr.collectReads(e.Y, stmts)
	case *ast.CallExpr:
		// Don't instrument the function itself if it's a simple identifier or selector
		// Only instrument if it's a function value from a variable
		switch fun := e.Fun.(type) {
		case *ast.Ident:
			// Simple function call - don't instrument the function name
		case *ast.SelectorExpr:
			// Package.Function or obj.Method - check if X is a package
			if instr.typeInfo != nil {
				if ident, ok := fun.X.(*ast.Ident); ok {
					if obj := instr.typeInfo.Uses[ident]; obj != nil {
						if _, isPkg := obj.(*types.PkgName); isPkg {
							// It's a package selector - don't instrument
							break
						}
						// Not a package, it's a real object - instrument it
						instr.collectReads(fun.X, stmts)
					}
					// obj is nil - unknown, assume it's a package (conservative)
				} else {
					// X is not a simple ident (e.g., obj.field.method())
					instr.collectReads(fun.X, stmts)
				}
			} else {
				// No type info - be conservative
				// If X is a simple identifier, it's likely a package, skip it
				if _, isIdent := fun.X.(*ast.Ident); !isIdent {
					// Not a simple ident, could be obj.method()
					instr.collectReads(fun.X, stmts)
				}
			}
		default:
			// Function expression - instrument it
			instr.collectReads(e.Fun, stmts)
		}
		// Instrument all arguments
		for _, arg := range e.Args {
			instr.collectReads(arg, stmts)
		}
	case *ast.ParenExpr:
		instr.collectReads(e.X, stmts)
	case *ast.SliceExpr:
		instr.collectReads(e.X, stmts)
		if e.Low != nil {
			instr.collectReads(e.Low, stmts)
		}
		if e.High != nil {
			instr.collectReads(e.High, stmts)
		}
		if e.Max != nil {
			instr.collectReads(e.Max, stmts)
		}
	case *ast.TypeAssertExpr:
		instr.collectReads(e.X, stmts)
	case *ast.IndexListExpr:
		instr.collectReads(e.X, stmts)
		for _, idx := range e.Indices {
			instr.collectReads(idx, stmts)
		}
	case *ast.BasicLit, *ast.FuncLit, *ast.CompositeLit:
		// Literals don't involve memory reads
	}
}

func (instr *Instrumenter) collectWrites(expr ast.Expr, stmts *[]ast.Stmt) {
	if expr == nil {
		return
	}

	switch e := expr.(type) {
	case *ast.Ident:
		*stmts = append(*stmts, &ast.ExprStmt{X: instr.makeMemWriteCall(e)})
	case *ast.SelectorExpr:
		// For writes to obj.field, we need to read obj first
		var readStmts []ast.Stmt
		instr.collectReads(e.X, &readStmts)
		*stmts = append(*stmts, readStmts...)
		*stmts = append(*stmts, &ast.ExprStmt{X: instr.makeMemWriteCall(e)})
	case *ast.IndexExpr:
		// For writes to arr[i] or m[key], we need to read arr/m and i/key first
		var readStmts []ast.Stmt
		instr.collectReads(e.X, &readStmts)
		instr.collectReads(e.Index, &readStmts)
		*stmts = append(*stmts, readStmts...)

		// Use type information to check if this is addressable
		if instr.typeInfo != nil {
			if tv, ok := instr.typeInfo.Types[e.X]; ok {
				// If it's a map, we can't take address of the element
				if _, isMap := tv.Type.Underlying().(*types.Map); !isMap {
					// It's an array or slice, we can write to the element
					*stmts = append(*stmts, &ast.ExprStmt{X: instr.makeMemWriteCall(e)})
				}
			}
		}
	case *ast.StarExpr:
		// For writes to *ptr, we need to read ptr first
		var readStmts []ast.Stmt
		instr.collectReads(e.X, &readStmts)
		*stmts = append(*stmts, readStmts...)
		*stmts = append(*stmts, &ast.ExprStmt{X: instr.makeMemWriteCall(e)})
	case *ast.ParenExpr:
		instr.collectWrites(e.X, stmts)
	}
}

func isBuiltin(name string) bool {
	builtins := map[string]bool{
		"append": true, "cap": true, "close": true, "complex": true,
		"copy": true, "delete": true, "imag": true, "len": true,
		"make": true, "new": true, "panic": true, "print": true,
		"println": true, "real": true, "recover": true,
		"true": true, "false": true, "nil": true, "iota": true,
	}
	return builtins[name]
}

// instrumentMainFunction adds GoroutineEnter/Exit hooks to main() in main package
func (instr *Instrumenter) instrumentMainFunction(f *ast.File) {
	// Only instrument if this is the main package
	if f.Name.Name != "main" {
		return
	}

	// Find the main function
	for _, decl := range f.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		// Check if this is the main function
		if funcDecl.Name.Name == "main" && funcDecl.Recv == nil {
			// Add GoroutineEnter at the beginning
			enterCall := &ast.ExprStmt{
				X: &ast.CallExpr{
					Fun: &ast.SelectorExpr{
						X:   &ast.Ident{Name: instr.config.RuntimeAlias},
						Sel: &ast.Ident{Name: instr.config.GoroutineEnterFunc},
					},
				},
			}

			// Add GoroutineExit at the end
			exitCall := &ast.ExprStmt{
				X: &ast.CallExpr{
					Fun: &ast.SelectorExpr{
						X:   &ast.Ident{Name: instr.config.RuntimeAlias},
						Sel: &ast.Ident{Name: instr.config.GoroutineExitFunc},
					},
				},
			}

			// Prepend enter call to the body
			if funcDecl.Body != nil {
				funcDecl.Body.List = append([]ast.Stmt{enterCall}, funcDecl.Body.List...)
				// Append exit call to the body
				funcDecl.Body.List = append(funcDecl.Body.List, exitCall)
				instr.instrumented = true
			}

			break
		}
	}
}
