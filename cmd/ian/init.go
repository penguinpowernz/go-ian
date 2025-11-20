package main

import (
	"github.com/penguinpowernz/go-ian"
	"github.com/penguinpowernz/go-ian/util/tell"
	"github.com/spf13/cobra"
)

func init() {
	addFieldFlags(initCmd)
	rootCmd.AddCommand(initCmd)
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the current folder as a Debian package",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		tell.IfFatalf(ian.Initialize(DIR), "")

		PKG = readPkg(DIR)

		if updatePkgFromFlags(cmd.Flag) {
			err := PKG.Ctrl().WriteFile(PKG.CtrlFile())
			tell.IfFatalf(err, "failed to save file after updating fields from flags")
		}
	},
}
