package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	ian "github.com/penguinpowernz/go-ian"
	"github.com/penguinpowernz/go-ian/util/tell"
	"github.com/spf13/cobra"
)

func init() {
	migrateCmd.Flags().BoolP("force", "f", false, "actually write the changes instead of only showing them")
	rootCmd.AddCommand(migrateCmd)
}

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate a pre-manifest package to the DEBIAN/md5sums manifest",
	Long: `Register the files of an older ian package so that it builds under the manifest
format, where DEBIAN/md5sums decides what goes into the package.

Every file the old packager would have included is registered in the manifest,
using the same exclude rules it used: the patterns in .ianignore plus ian's
built in defaults (.git, pkg, .gitignore, .ianpush, .ianignore, .gitkeep and
hidden files in the package root).

Bare files in the package root are registered as doc files instead, in
DEBIAN/docfiles, because the old packager swept them into
usr/share/doc/<package> rather than installing them at the root.

Migration only shows what it would do; pass -f to write the changes.  It refuses
to run when files are already registered, since the manifest is then the source
of truth and re-deriving it from the old ignore rules would be wrong.  Writing
the changes also deletes .ianignore, which the manifest format does not use.`,
	Run: func(cmd *cobra.Command, args []string) {
		PKG = readPkg(DIR)

		force, _ := cmd.Flags().GetBool("force")

		mp, err := PKG.PlanMigration()
		tell.IfFatalf(err, "failed to plan the migration")

		// once anything is registered the manifest is authoritative, so there is
		// nothing safe to migrate into
		if mp.AlreadyRegistered {
			tell.Fatalf("%s already has registered files, nothing to migrate", PKG.ManifestFile())
		}

		if mp.Empty() {
			fmt.Println("Nothing to migrate: no includable files found.")
			return
		}

		printMigrationPlan(mp)

		if !force {
			fmt.Println("Nothing written. Re-run with -f to apply the above.")
			if mp.Legacy != "" {
				fmt.Printf("\n  NOTE: applying this will DELETE %s, which the manifest format does not use.\n", mp.Legacy)
				fmt.Println("        Its patterns are baked into the file list above, so review that list first.")
			}
			return
		}

		tell.IfFatalf(PKG.ApplyMigration(mp), "migration failed")

		// the manifest is only useful if it actually verifies, so prove it
		problems, err := PKG.Verify(false)
		for _, p := range problems {
			fmt.Fprintln(os.Stderr, "WARNING: "+p)
		}
		tell.IfFatalf(err, "the migrated manifest does not verify")

		fmt.Printf("Registered %d file(s) and %d doc file(s).\n", len(mp.Files), len(mp.Docs))

		if mp.Legacy != "" {
			tell.IfFatalf(os.Remove(mp.Legacy), "failed to remove %s", mp.Legacy)
			fmt.Println("Deleted", mp.Legacy)
		}

		fmt.Println("  (review the changes, then commit DEBIAN/md5sums and DEBIAN/docfiles)")
	},
}

// printMigrationPlan shows what the migration will register
func printMigrationPlan(mp ian.MigrationPlan) {
	if len(mp.Files) > 0 {
		fmt.Println("Will be registered in the manifest:")
		fmt.Println()
		for _, f := range mp.Files {
			fmt.Println("\t" + f)
		}
		fmt.Println()
	}

	if len(mp.Docs) > 0 {
		fmt.Println("Will be registered as doc files:")
		fmt.Println("  (bare root files, which the old packager moved into the doc dir)")
		fmt.Println()
		w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
		for _, d := range mp.Docs {
			fmt.Fprintf(w, "\t%s\t-> %s\n", d, PKG.DocDest(d))
		}
		w.Flush()
		fmt.Println()
	}

	if len(mp.Excluded) > 0 {
		fmt.Println("Excluded by the old ignore rules:")
		fmt.Println()
		for _, e := range mp.Excluded {
			fmt.Println("\t" + e)
		}
		fmt.Println()
	}
}
