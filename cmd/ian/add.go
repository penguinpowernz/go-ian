package main

import (
	"github.com/penguinpowernz/go-ian/util/tell"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(addCmd)
}

var addCmd = &cobra.Command{
	Use:   "add <file>...",
	Short: "Register files to be included in the package",
	Long: `Compute the MD5 sum of each given file and add (or update) its entry in
DEBIAN/md5sums.  Only files registered this way are included when building the
package.  Paths are relative to the package directory.

Directories are walked recursively, so "ian add ." registers every file in the
package.  The DEBIAN and pkg directories, VCS metadata and ian's own dotfiles
are never registered.`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		PKG = readPkg(DIR)

		files, err := PKG.ExpandFiles(args)
		tell.IfFatalf(err, "failed to expand the given paths")

		if len(files) == 0 {
			tell.Fatalf("no files found to add")
		}

		for _, f := range files {
			tell.IfFatalf(PKG.AddFile(f), "failed to add %s", f)
		}
	},
}
