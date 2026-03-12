package main

import (
	"fmt"

	"github.com/penguinpowernz/go-ian/util/tell"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(filesCmd)
}

var filesCmd = &cobra.Command{
	Use:   "files",
	Short: "List files that would be included in the package",
	Long:  `List files that would be included in the package, using the same exclude rules as the build`,
	Run: func(cmd *cobra.Command, args []string) {
		PKG = readPkg(DIR)

		files, err := PKG.ListFiles()
		tell.IfFatalf(err, "failed to list package files")
		for _, f := range files {
			fmt.Println(f)
		}
	},
}
