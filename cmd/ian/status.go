package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	ian "github.com/penguinpowernz/go-ian"
	"github.com/penguinpowernz/go-ian/util/colour"
	"github.com/penguinpowernz/go-ian/util/tell"
	"github.com/spf13/cobra"
)

func init() {
	statusCmd.Flags().BoolP("all", "a", false, "also list the registered files that have not changed")
	statusCmd.Flags().BoolP("verbose", "v", false, "show the md5 sums of each file")
	statusCmd.Flags().BoolP("quiet", "q", false, "print nothing, only set the exit code")
	rootCmd.AddCommand(statusCmd)
}

// statusLabel is the short marker shown against each file in the status listing
func statusLabel(s ian.FileState) string {
	switch s {
	case ian.StateModified:
		return "modified"
	case ian.StateMissing:
		return "missing"
	case ian.StateError:
		return "error"
	}
	return "ok"
}

// statusColour returns the escape for a state: green for a file that still
// matches the manifest, yellow for one that has drifted, red for one that is
// gone or unreadable
func statusColour(s ian.FileState) string {
	switch s {
	case ian.StateModified:
		return colour.Yellow
	case ian.StateMissing, ian.StateError:
		return colour.Red
	}
	return colour.Green
}

// labelWidth is the width of the status column, wide enough for the longest
// label so the paths line up down the listing.  sumWidth is the width of an
// md5 sum, for the column it occupies under -v.
const (
	labelWidth = 8
	sumWidth   = 32
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show changed and unregistered files",
	Long: `Show what differs between the repo and DEBIAN/md5sums: registered files that
have been modified or gone missing, and files in the repo that are not
registered and so will not be included in the package.  Exits non-zero when any
registered file has drifted, so it can be used as a check in CI.

Registered files that still match the manifest are only counted.  Use -a to
list them as well.

Unregistered files are the ones "ian add ." would register.  A directory with
nothing registered in it is shown once rather than file by file.  They do not
affect the exit code, since a file left out of the package is not an error.

Use -v to show the recorded sum of each file, and the current sum alongside it
where the two differ.`,
	Run: func(cmd *cobra.Command, args []string) {
		PKG = readPkg(DIR)

		all, _ := cmd.Flags().GetBool("all")
		verbose, _ := cmd.Flags().GetBool("verbose")
		quiet, _ := cmd.Flags().GetBool("quiet")

		c := colour.For(os.Stdout)

		statuses, err := PKG.Status()
		tell.IfFatalf(err, "failed to read manifest")

		drifted := 0
		for _, st := range statuses {
			if st.State != ian.StateOK {
				drifted++
			}
		}

		if quiet {
			if drifted > 0 {
				os.Exit(1)
			}
			return
		}

		unregistered, err := PKG.Unregistered()
		tell.IfFatalf(err, "failed to look for unregistered files")

		// with per-arch manifests there is more than one to choose from, so say
		// which one these sums came from whenever it is not simply "md5sums"
		if name, fallback := PKG.ManifestSource(); name != ian.ManifestName || fallback {
			line := fmt.Sprintf("Manifest: DEBIAN/%s (%s)", name, PKG.Ctrl().Arch)
			if fallback {
				line = fmt.Sprintf("Manifest: DEBIAN/%s, which has no manifest of its own for %s",
					name, PKG.Ctrl().Arch)
			}
			fmt.Println(c.P(colour.Dim, line))
			fmt.Println()
		}

		// group by state so each group can carry its own hint, the way git
		// status explains what to do about each section
		groups := []struct {
			state ian.FileState
			title string
			hints []string
		}{
			{
				state: ian.StateModified,
				title: "Changed since registered:",
				hints: []string{
					`(use "ian add <file>..." to record the new sum)`,
					`(use "ian rm <file>..." to drop it from the package)`,
				},
			},
			{
				state: ian.StateMissing,
				title: "Registered but missing from the repo:",
				hints: []string{
					`(use "ian rm <file>..." to unregister it)`,
				},
			},
			{
				state: ian.StateError,
				title: "Could not be read:",
			},
			{
				state: ian.StateOK,
				title: "Registered and unchanged:",
			},
		}

		for _, g := range groups {
			// the unchanged files are the bulk of any package and say nothing
			// about what needs doing, so they are only listed on request
			if g.state == ian.StateOK && !all {
				continue
			}

			var in []ian.FileStatus
			for _, st := range statuses {
				if st.State == g.state {
					in = append(in, st)
				}
			}
			if len(in) == 0 {
				continue
			}

			fmt.Println(c.P(colour.Bold, g.title))
			for _, h := range g.hints {
				fmt.Println("  " + c.P(colour.Dim, h))
			}
			fmt.Println()

			w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
			for _, st := range in {
				// name the repo file, noting where a doc file installs to
				shown := st.Path
				if st.IsDoc() {
					shown = fmt.Sprintf("%s -> %s", st.Source, st.Path)
				}

				// pad the label before painting it: tabwriter measures cells
				// by their bytes and would count the escapes as width
				label := c.Pad(statusColour(st.State), statusLabel(st.State), labelWidth)

				switch {
				case !verbose:
					fmt.Fprintf(w, "\t%s\t%s\n", label, shown)
				case st.State == ian.StateModified:
					// show what was recorded and what is actually there now
					fmt.Fprintf(w, "\t%s\t%s\t%s\t%s\n", label, shown,
						c.Pad(colour.Dim, st.Want, sumWidth), c.P(colour.Yellow, "now "+st.Got))
				case st.State == ian.StateError:
					fmt.Fprintf(w, "\t%s\t%s\t%s\t%s\n", label, shown,
						c.Pad(colour.Dim, st.Want, sumWidth), c.P(colour.Red, fmt.Sprint(st.Err)))
				default:
					fmt.Fprintf(w, "\t%s\t%s\t%s\n", label, shown, c.P(colour.Dim, st.Want))
				}
			}
			w.Flush()
			fmt.Println()
		}

		if len(unregistered) > 0 {
			fmt.Println(c.P(colour.Bold, "Not registered, so not in the package:"))
			fmt.Println("  " + c.P(colour.Dim, `(use "ian add <file>..." to include it in the package)`))
			fmt.Println()
			for _, f := range unregistered {
				fmt.Println("  " + c.P(colour.Red, f))
			}
			fmt.Println()
		}

		if len(statuses) == 0 {
			fmt.Println(c.P(colour.Bold, "No files registered for packaging."))
			fmt.Println("  " + c.P(colour.Dim, `(use "ian add <file>..." to register files to include in the package)`))
			return
		}

		if drifted == 0 {
			fmt.Println(c.P(colour.Green, fmt.Sprintf("%d file(s) registered, all match the manifest.", len(statuses))))
			hint := `(use "ian status -a" to list them, "ian pkg" to build)`
			if all {
				hint = `(use "ian add <file>..." to register more, "ian pkg" to build)`
			}
			fmt.Println("  " + c.P(colour.Dim, hint))
			return
		}

		fmt.Println(c.P(colour.Red, fmt.Sprintf("%d of %d registered file(s) have drifted.", drifted, len(statuses))))
		fmt.Println("  " + c.P(colour.Dim, `("ian pkg" will refuse to build until this is resolved, or pass -k to build anyway)`))
		os.Exit(1)
	},
}
