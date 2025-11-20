package main

import (
	"fmt"

	"github.com/penguinpowernz/go-ian"
	"github.com/penguinpowernz/go-ian/util/tell"
	"github.com/spf13/cobra"
)

func init() {
	pkgCmd.Flags().StringP("outpath", "o", "", "output path (the push command won't see files in this dir)")
	pkgCmd.Flags().BoolP("debug", "x", false, "debug mode")
	rootCmd.AddCommand(pkgCmd)
}

var pkgCmd = &cobra.Command{
	Use:   "pkg",
	Short: "Generate the package file",
	Long:  `Generate the package file, printing the package location on success`,
	Run: func(cmd *cobra.Command, args []string) {
		PKG = readPkg(DIR)

		outpath := cmd.Flag("outpath").Value.String()
		debug, _ := cmd.Flags().GetBool("debug")

		pkgr := ian.DefaultPackager()
		outfile, err := pkgr.BuildWithOpts(PKG, ian.BuildOpts{Outpath: outpath, Debug: debug})
		tell.IfFatalf(err, "packaging failed")
		fmt.Println(outfile)
	},
}
