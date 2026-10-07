package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/penguinpowernz/go-ian/util/tell"
	"github.com/spf13/cobra"
)

func init() {
	docCmd.Flags().BoolP("remove", "r", false, "unregister the given doc files instead of adding them")
	rootCmd.AddCommand(docCmd)
}

var docCmd = &cobra.Command{
	Use:   "doc [file]...",
	Short: "List or register files installed to the package doc directory",
	Long: `With no arguments, list the files registered in DEBIAN/docfiles along with the
location each one installs to inside the package.

Given one or more files, register them as doc files: the path is recorded in
DEBIAN/docfiles and its destination and MD5 sum are added to DEBIAN/md5sums, so
the file is included in the package and covered by verification.

Doc files install into usr/share/doc/<package> under their base name, so
"ian doc docs/guide.md" installs as usr/share/doc/<package>/guide.md.  This
replaces the old behaviour of sweeping every bare file in the repo root into the
doc directory.

Use -r to unregister a doc file, which leaves the file itself on disk.`,
	Run: func(cmd *cobra.Command, args []string) {
		PKG = readPkg(DIR)

		remove, _ := cmd.Flags().GetBool("remove")

		if len(args) == 0 {
			if remove {
				tell.Fatalf("no files given to remove")
			}
			listDocFiles()
			return
		}

		for _, f := range args {
			if remove {
				tell.IfFatalf(PKG.RemoveDocFile(f), "failed to remove %s", f)
				fmt.Println("removed", f)
				continue
			}
			tell.IfFatalf(PKG.AddDocFile(f), "failed to add %s", f)
			fmt.Printf("%s -> %s\n", f, PKG.DocDest(f))
		}
	},
}

// listDocFiles prints each registered doc file and where it installs to
func listDocFiles() {
	docs, err := PKG.DocFiles()
	tell.IfFatalf(err, "failed to read doc files")

	if len(docs) == 0 {
		fmt.Println("No doc files registered.")
		fmt.Println("  (use \"ian doc <file>...\" to install a file to " + PKG.DocDir() + ")")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	fmt.Fprintln(w, "FILE\tINSTALLS AS")
	for _, d := range docs {
		fmt.Fprintf(w, "%s\t%s\n", d, PKG.DocDest(d))
	}
	w.Flush()
}
