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
	pkgCmd.Flags().CountP("quiet", "q", "suppress the md5sums listing, -qq also suppresses the package filename")
	pkgCmd.Flags().BoolP("file-list", "f", false, "print file list to stderr instead of md5sums")
	pkgCmd.Flags().BoolP("insecure", "k", false, "skip md5sum verification (warn instead of failing)")
	pkgCmd.Flags().BoolP("no-package-check", "K", false, "skip rechecking the built package against the manifest")
	rootCmd.AddCommand(pkgCmd)
}

var pkgCmd = &cobra.Command{
	Use:   "pkg",
	Short: "Generate the package file",
	Long:  `Generate the package file, printing the package location on success`,
	Run: func(cmd *cobra.Command, args []string) {
		PKG = readPkg(DIR)

		dryRun, _ := cmd.Flags().GetBool("dry-run")
		quietCount, _ := cmd.Flags().GetCount("quiet")
		debug, _ := cmd.Flags().GetBool("debug")
		fileList, _ := cmd.Flags().GetBool("file-list")
		insecure, _ := cmd.Flags().GetBool("insecure")
		noPkgCheck, _ := cmd.Flags().GetBool("no-package-check")

		// -q silences the md5sums listing, -qq also silences the filename
		quiet := quietCount > 0
		silent := quietCount > 1

		if dryRun || (fileList && !quiet && !debug) {
			m, err := PKG.Manifest()
			tell.IfFatalf(err, "failed to read manifest")
			for _, f := range m.Paths() {
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
		outfile, err := pkgr.BuildWithOpts(PKG, ian.BuildOpts{
			Outpath:          outpath,
			Debug:            debug,
			Quiet:            quiet || fileList,
			Insecure:         insecure,
			SkipPackageCheck: noPkgCheck,
		})
		tell.IfFatalf(err, "packaging failed")

		if !silent {
			fmt.Println(outfile)
		}
	},
}
