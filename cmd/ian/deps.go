package main

import (
	"fmt"
	"strings"

	"github.com/penguinpowernz/go-ian/util/str"
	"github.com/penguinpowernz/go-ian/util/tell"
	"github.com/spf13/cobra"
)

func init() {
	depsCmd.Flags().StringP("add", "a", "", "add a dependency")
	depsCmd.Flags().StringP("remove", "r", "", "remove a dependency")
	rootCmd.AddCommand(depsCmd)
}

var depsCmd = &cobra.Command{
	Use:   "deps",
	Short: "Manage dependencies",
	Long:  `Add or remove dependencies, or show the dependencies by omitting arguments`,
	Run: func(cmd *cobra.Command, args []string) {
		PKG = readPkg(DIR)

		add, _ := cmd.Flags().GetString("add")
		remove, _ := cmd.Flags().GetString("remove")

		if add == "" && remove == "" {
			for _, deps := range PKG.Ctrl().Depends {
				fmt.Println(deps)
			}

			return
		}

		if add != "" {
			newpkgs := str.CleanStrings(strings.Split(add, ","))
			PKG.Ctrl().Depends = append(PKG.Ctrl().Depends, newpkgs...)
		}

		if remove != "" {
			rmdeps := str.CleanStrings(strings.Split(remove, ","))

			deps := strings.Join(PKG.Ctrl().Depends, " ")
			deps = " " + deps + " "
			for _, dep := range rmdeps {
				deps = strings.Replace(deps, " "+dep+" ", " ", -1)
			}

			PKG.Ctrl().Depends = str.CleanStrings(strings.Split(deps, " "))
		}

		err := PKG.Ctrl().WriteFile(PKG.CtrlFile())
		tell.IfFatalf(err, "couldn't set fields")
	},
}
