package main

import (
	"os"

	ian "github.com/penguinpowernz/go-ian"
	"github.com/penguinpowernz/go-ian/util/tell"
	"github.com/penguinpowernz/md5walk"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(md5sumsCmd)
}

var md5sumsCmd = &cobra.Command{
	Use:   "md5",
	Short: "Show MD5 sums of the files that would be packaged",
	Long:  `Stage files as if building a package, then print MD5 sums to stdout without finalizing the build`,
	Run: func(cmd *cobra.Command, args []string) {
		PKG = readPkg(DIR)

		br := &ian.BuildRequest{Pkg: PKG}
		tell.IfFatalf(ian.StageFiles(br), "failed to stage files")
		defer br.CleanUp()

		tell.IfFatalf(ian.CleanRoot(br), "failed to clean root")

		sums, err := md5walk.Walk(br.Tmp)
		tell.IfFatalf(err, "failed to calculate MD5 sums")

		_, err = sums.Write(os.Stdout)
		tell.IfFatalf(err, "failed to write MD5 sums")
	},
}
