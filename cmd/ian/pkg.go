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
	pkgCmd.Flags().BoolP("quiet", "q", false, "suppress output when building")
	pkgCmd.Flags().BoolP("file-list", "f", false, "print file list to stderr instead of md5sums")
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
		fileList, _ := cmd.Flags().GetBool("file-list")

		if dryRun || (fileList && !quiet && !debug) {
			files, err := PKG.ListFiles()
			tell.IfFatalf(err, "failed to list package files")
			for _, f := range files {
				fmt.Fprintln(cmd.ErrOrStderr(), f)
			}
			if dryRun {
				return
			}
		}

		outpathv := cmd.Flag("outpath").Value
		var outpath string
		if outpathv != nil {
			outpath = outpathv.String()
		}

		pkgr := ian.DefaultPackager()
		outfile, err := pkgr.BuildWithOpts(PKG, ian.BuildOpts{Outpath: outpath, Debug: debug, PrintMD5Sums: !quiet && !fileList})
		tell.IfFatalf(err, "packaging failed")
		fmt.Println(outfile)
	},
}
