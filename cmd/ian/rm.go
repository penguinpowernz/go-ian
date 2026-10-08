package main

import (
	"fmt"
	"strings"

	"github.com/penguinpowernz/go-ian/util/tell"
	"github.com/spf13/cobra"
)

func init() {
	rmCmd.Flags().BoolP("all-arches", "a", false, "unregister the file from the manifests of every architecture")
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
every file.  Entries whose files have already been deleted can still be removed.

A file registered as a doc file is unregistered by its path in the repo, the
same one "ian add" takes, and is dropped from DEBIAN/docfiles along with its
entry under the doc directory.

Use -a to unregister the file from every architecture's manifest at once, rather
than only the one for the architecture in the control file.  This is the
counterpart to "ian add -a": a file that was registered everywhere otherwise
takes one "ian set -a" and "ian rm" per architecture to take back out.  An
argument only has to match in one manifest, so a file that some architectures
carry and others do not can still be removed in one go.`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		PKG = readPkg(DIR)

		if all, _ := cmd.Flags().GetBool("all-arches"); all {
			runRemoveAllArches(args)
			return
		}

		removed, err := PKG.RemoveFiles(args)
		tell.IfFatalf(err, "failed to remove files from the manifest")

		for _, f := range removed {
			fmt.Println("removed", f)
		}
	},
}

// runRemoveAllArches unregisters the given files from every architecture's
// manifest, naming the manifests written so it is clear the removal went
// further than the current architecture
func runRemoveAllArches(args []string) {
	removed, written, err := PKG.RemoveFilesAllArches(args)
	tell.IfFatalf(err, "failed to remove files from the manifests")

	for _, f := range removed {
		fmt.Printf("removed %s from %s\n", f, strings.Join(written, ", "))
	}
}
