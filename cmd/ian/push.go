package main

import (
	"github.com/penguinpowernz/go-ian"
	"github.com/penguinpowernz/go-ian/util/file"
	"github.com/penguinpowernz/go-ian/util/tell"
	"github.com/spf13/cobra"
)

func init() {
	pushCmd.PersistentFlags().StringP("target", "t", "", "set the target to push to")
	rootCmd.AddCommand(pushCmd)
}

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push a package to a remote repository",
	Long:  `Push a package to a remote repository using the .ianpush file`,
	Run: func(cmd *cobra.Command, args []string) {
		PKG = readPkg(DIR)

		if !file.Exists(PKG.PushFile()) {
			tell.Fatalf("No .ianpush file exists")
		}

		deb := PKG.DebFile()

		target, _ := cmd.Flags().GetString("target")
		err := ian.Push(PKG.PushFile(), deb, target)
		tell.IfFatalf(err, "pushing failed")
	},
}
