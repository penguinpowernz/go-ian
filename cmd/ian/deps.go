package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/penguinpowernz/go-ian/util/colour"
	"github.com/penguinpowernz/go-ian/util/str"
	"github.com/penguinpowernz/go-ian/util/tell"
	"github.com/spf13/cobra"
)

func init() {
	depsCmd.Flags().StringP("add", "a", "", "add a dependency (comma separated for more than one)")
	depsCmd.Flags().StringP("remove", "r", "", "remove a dependency (comma separated for more than one)")
	rootCmd.AddCommand(depsCmd)
}

var depsCmd = &cobra.Command{
	Use:   "deps",
	Short: "Manage dependencies",
	Long: `Add or remove dependencies, or show the dependencies by omitting arguments.

These are the packages listed in the Depends field of DEBIAN/control, which apt
installs alongside this package.  Given no flags, each one is printed on its own
line.

Both flags take a comma separated list, so several can be managed at once, and
each entry is a dpkg dependency specification: a package name on its own, or a
name with a version constraint or an alternative.  Because the separator is a
comma, a constraint may contain spaces as usual.

A dependency is removed by giving the exact text it was added with, constraint
and all; removing one that isn't there is not an error.  Adding one that is
already listed leaves the list unchanged, so -a can be run repeatedly.

Examples:

    ian deps                            # list the current dependencies
    ian deps -a curl                    # depend on curl
    ian deps -a "curl,jq"               # depend on both
    ian deps -a "bash (>= 4.0)"         # depend on a version of bash
    ian deps -a "nginx | apache2"       # depend on either one
    ian deps -r curl                    # drop the dependency on curl
    ian deps -r "bash (>= 4.0)"         # the constraint is part of the name

Use "ian set" for the rest of the control file fields.`,
	Run: func(cmd *cobra.Command, args []string) {
		PKG = readPkg(DIR)

		add, _ := cmd.Flags().GetString("add")
		remove, _ := cmd.Flags().GetString("remove")

		if add == "" && remove == "" {
			listDeps()
			return
		}

		if add != "" {
			newpkgs := str.CleanStrings(strings.Split(add, ","))
			PKG.Ctrl().Depends = appendDeps(PKG.Ctrl().Depends, newpkgs)
		}

		if remove != "" {
			rmdeps := str.CleanStrings(strings.Split(remove, ","))
			PKG.Ctrl().Depends = removeDeps(PKG.Ctrl().Depends, rmdeps)
		}

		err := PKG.Ctrl().WriteFile(PKG.CtrlFile())
		tell.IfFatalf(err, "couldn't set fields")
	},
}

// listDeps prints the dependencies one per line on stdout, with the header and
// hint on stderr in the style of "ian status", so that the list stays usable in
// a pipe while a reader at the terminal still sees what changes it
func listDeps() {
	deps := PKG.Ctrl().Depends
	c := colour.For(os.Stderr)

	if len(deps) == 0 {
		fmt.Fprintln(os.Stderr, c.P(colour.Bold, "No dependencies."))
		fmt.Fprintln(os.Stderr, "  "+c.P(colour.Dim, `(use "ian deps -a curl" to add one, quoting a version like "bash (>= 4.0)")`))
		return
	}

	fmt.Fprintln(os.Stderr, c.P(colour.Bold, "Dependencies:"))
	fmt.Fprintln(os.Stderr, "  "+c.P(colour.Dim, `(use "ian deps -a <pkg>[,<pkg>...]" to add, "ian deps -r <pkg>" to remove)`))
	fmt.Fprintln(os.Stderr)

	for _, dep := range deps {
		fmt.Println(dep)
	}
}

// appendDeps adds the given dependencies to the list, ignoring empty entries
// and any that are already listed so that -a can be run more than once
func appendDeps(deps, add []string) []string {
	for _, dep := range add {
		if dep == "" || depIndex(deps, dep) > -1 {
			continue
		}

		deps = append(deps, dep)
	}

	return deps
}

// removeDeps drops the given dependencies from the list, matching them in full
// so that a version constraint is matched along with the package name
func removeDeps(deps, remove []string) []string {
	for _, dep := range remove {
		if dep == "" {
			continue
		}

		if i := depIndex(deps, dep); i > -1 {
			deps = append(deps[:i], deps[i+1:]...)
		}
	}

	return deps
}

// depIndex finds a dependency in the list, ignoring the spacing around it as
// the control file may have been written by hand
func depIndex(deps []string, dep string) int {
	for i, d := range deps {
		if strings.TrimSpace(d) == strings.TrimSpace(dep) {
			return i
		}
	}

	return -1
}
