package main

import (
	"fmt"

	"github.com/penguinpowernz/go-ian/util/tell"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(rmCmd)
}

var rmCmd = &cobra.Command{
	Use:     "rm <file>...",
	Aliases: []string{"remove"},
	Short:   "Unregister files so they are no longer included in the package",
	Long: `Remove each given file's entry from DEBIAN/md5sums so that it is no longer
included when building the package.  The files themselves are left on disk.
Paths are relative to the package directory.

Naming a directory unregisters everything beneath it, and "ian rm ." unregisters
every file.  Entries whose files have already been deleted can still be removed.`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		PKG = readPkg(DIR)

		removed, err := PKG.RemoveFiles(args)
		tell.IfFatalf(err, "failed to remove files from the manifest")

		for _, f := range removed {
			fmt.Println("removed", f)
		}
	},
}
