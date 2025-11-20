package main

import (
	"fmt"
	"log"
	"os"

	"github.com/penguinpowernz/go-ian"
	"github.com/penguinpowernz/go-ian/util/tell"
	"github.com/spf13/cobra"
)

var (
	PKG     *ian.Pkg
	DIR     = "."
	Version string
)

func init() {
	if v := os.Getenv("IAN_DIR"); v != "" {
		DIR = v
	}

	rootCmd.PersistentFlags().StringVarP(&DIR, "dir", "d", DIR, "the directory of the package (also set by IAN_DIR envvar)")
	rootCmd.PersistentFlags().BoolP("version", "V", false, "show the version")
}

var rootCmd = &cobra.Command{
	Use:   "ian",
	Short: "",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {

		if v, _ := cmd.Flags().GetBool("version"); v {
			fmt.Println("Version", Version)
			fmt.Println("In memory of Ian Ashley Murdock (1973 - 2015), founder of the Debian Project")
			return
		}

		cmd.Help()
	},
}

func readPkg(dir string) *ian.Pkg {
	ensureInit(dir)
	pkg, err := ian.NewPackage(dir)
	tell.IfFatalf(err, "couldn't read control file")
	return pkg
}

func ensureInit(dir string) {
	if !ian.IsInitialized(dir) {
		log.Fatal("not initialized")
	}
}
