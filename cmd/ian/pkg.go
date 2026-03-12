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
	pkgCmd.Flags().BoolP("dry-run", "n", false, "print files that would be included without building")
	pkgCmd.Flags().BoolP("quiet", "q", false, "suppress file list when building")
	rootCmd.AddCommand(pkgCmd)
}

var pkgCmd = &cobra.Command{
	Use:   "pkg",
	Short: "Generate the package file",
	Long:  `Generate the package file, printing the package location on success`,
	Run: func(cmd *cobra.Command, args []string) {
		PKG = readPkg(DIR)

		dryRun, _ := cmd.Flags().GetBool("dry-run")
		quiet, _ := cmd.Flags().GetBool("quiet")
		debug, _ := cmd.Flags().GetBool("debug")

		if dryRun || (!quiet && !debug) {
			files, err := PKG.ListFiles()
			tell.IfFatalf(err, "failed to list package files")
			for _, f := range files {
				fmt.Println(f)
			}
			if dryRun {
				return
			}
		}

		outpath := cmd.Flag("outpath").Value.String()

		pkgr := ian.DefaultPackager()
		outfile, err := pkgr.BuildWithOpts(PKG, ian.BuildOpts{Outpath: outpath, Debug: debug})
		tell.IfFatalf(err, "packaging failed")
		fmt.Println(outfile)
	},
}
