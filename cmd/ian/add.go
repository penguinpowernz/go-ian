package main

import (
	"fmt"
	"os"

	"github.com/penguinpowernz/go-ian/util/colour"
	"github.com/penguinpowernz/go-ian/util/tell"
	"github.com/spf13/cobra"
)

func init() {
	addCmd.Flags().BoolP("update", "u", false, "re-register every registered file that has changed")
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
are never registered.

Use -u to re-record the sums of all already registered files that have changed,
without naming any of them: it updates everything "ian status" reports as
modified.  Registered files that have gone missing are reported and left alone,
as they need restoring or "ian rm" rather than a new sum.`,
	Args: func(cmd *cobra.Command, args []string) error {
		// -u takes its file list from the manifest, so it both needs no
		// arguments and would be ambiguous alongside them
		if update, _ := cmd.Flags().GetBool("update"); update {
			if len(args) > 0 {
				return fmt.Errorf("-u updates all changed files, so it takes no file arguments")
			}
			return nil
		}
		return cobra.MinimumNArgs(1)(cmd, args)
	},
	Run: func(cmd *cobra.Command, args []string) {
		PKG = readPkg(DIR)

		if update, _ := cmd.Flags().GetBool("update"); update {
			runUpdate()
			return
		}

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

// runUpdate re-sums every registered file that has drifted from the manifest,
// reporting what it updated and what it could not
func runUpdate() {
	c := colour.For(os.Stdout)

	updated, problems, err := PKG.UpdateFiles()
	tell.IfFatalf(err, "failed to update the manifest")

	for _, f := range updated {
		fmt.Println("updated", f)
	}

	for _, p := range problems {
		fmt.Fprintln(os.Stderr, c.P(colour.Red, p))
	}

	if len(updated) == 0 {
		fmt.Println("no registered files have changed")
	}

	// the manifest still does not describe the repo, so say so in the exit code
	// the same way "ian status" does
	if len(problems) > 0 {
		os.Exit(1)
	}
}
