package cmd

import (
	"go/printer"
	"go/token"
	"path/filepath"
	"errors"

	"github.com/amirkhaki/moriarty/pkg/instrument"
	"github.com/spf13/cobra"
	"os"
)

// instrumentCmd represents the instrument command
var instrumentCmd = &cobra.Command{
	Use:   "instrument",
	Short: "instrument given files",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(inputs) == 0 {
			return nil
		}
		instr := instrument.NewInstrumenter(nil)
		fset := token.NewFileSet()

		files, err := instr.InstrumentFiles(fset, inputs)
		if err != nil {
			return err
		}
		var jerr error
		for i := range inputs {
			f, s := files[i], inputs[i]
			dir, filename := filepath.Split(s)
			ext := filepath.Ext(filename)
			filename = filename[:len(filename)-len(ext)] + postfix + ext
			output := dir+filename
			if _, err := os.Stat(output); errors.Is(err, os.ErrNotExist) || force {
				file, err := os.Create(output)
				jerr = errors.Join(jerr, err)
				printer.Fprint(file, fset, f)
			}
		}
		return jerr
	},
}

var inputs []string
var postfix string
var force bool

func init() {
	rootCmd.AddCommand(instrumentCmd)

	instrumentCmd.Flags().StringArrayVarP(&inputs, "input", "i",
		[]string{}, "path of input files")
	instrumentCmd.Flags().StringVarP(&postfix, "postfix", "p", "_moriarty",
		"postfix of generated files (alongside input files)")
	instrumentCmd.Flags().BoolVarP(&force, "force", "f", false,
		"force override files")
}
