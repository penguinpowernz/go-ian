package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/penguinpowernz/go-ian/util/colour"
	"github.com/penguinpowernz/go-ian/util/tell"
	"github.com/spf13/cobra"
)

func init() {
	addCmd.Flags().BoolP("update", "u", false, "re-register every registered file that has changed")
	addCmd.Flags().BoolP("all-arches", "a", false, "register the file in the manifests of every architecture")
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

A file already registered as a doc file keeps its place in the doc directory:
its sum is updated against the entry it installs as, rather than the file being
registered a second time at its path in the repo.  This applies to "ian add ."
and -a as well, so a sweep over the whole package re-sums the doc files in it
instead of duplicating them.

Use -u to re-record the sums of all already registered files that have changed,
without naming any of them: it updates everything "ian status" reports as
modified.  Registered files that have gone missing are reported and left alone,
as they need restoring or "ian rm" rather than a new sum.

Use -a to register the file in every architecture's manifest at once, rather
than only the one for the architecture in the control file.  This is for files
that are the same on every architecture, such as a config file or a unit file.
Do not use it for a cross-compiled binary: those are different bytes per
architecture, so one sum cannot describe them all.`,
	Args: func(cmd *cobra.Command, args []string) error {
		// -u takes its file list from the manifest, so it both needs no
		// arguments and would be ambiguous alongside them
		if update, _ := cmd.Flags().GetBool("update"); update {
			if len(args) > 0 {
				return fmt.Errorf("-u updates all changed files, so it takes no file arguments")
			}
			// -u re-sums what each manifest already records, which is per
			// arch by definition, so there is nothing for -a to mean here
			if all, _ := cmd.Flags().GetBool("all-arches"); all {
				return fmt.Errorf("-u and -a cannot be combined: -u updates the manifest for the current architecture")
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

		if all, _ := cmd.Flags().GetBool("all-arches"); all {
			runAddAllArches(files)
			return
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

// runAddAllArches registers each file in every architecture's manifest, naming
// the manifests written so it is clear the file went further than the current
// architecture
func runAddAllArches(files []string) {
	for _, f := range files {
		written, err := PKG.AddFileAllArches(f)
		tell.IfFatalf(err, "failed to add %s", f)

		fmt.Printf("added %s to %s\n", f, strings.Join(written, ", "))
	}
}
