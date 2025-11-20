package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	infoCmd.Flags().BoolP("version", "v", false, "show the version")
	infoCmd.Flags().BoolP("priority", "p", false, "show the package priority")
	infoCmd.Flags().BoolP("section", "s", false, "show the package section")
	infoCmd.Flags().BoolP("architecture", "a", false, "show the package architecture")
	infoCmd.Flags().BoolP("depends", "x", false, "show the package dependencies")
	infoCmd.Flags().BoolP("maintainer", "m", false, "show the package maintainer")
	infoCmd.Flags().BoolP("homepage", "u", false, "show the package homepage")

	rootCmd.AddCommand(infoCmd)
}

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show information about the current package",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		PKG = readPkg(DIR)

		if v, _ := cmd.Flags().GetBool("version"); v {
			fmt.Println(PKG.Ctrl().Version)
			return
		}

		if v, _ := cmd.Flags().GetBool("priority"); v {
			fmt.Println(PKG.Ctrl().Priority)
			return
		}

		if v, _ := cmd.Flags().GetBool("section"); v {
			fmt.Println(PKG.Ctrl().Section)
			return
		}

		if v, _ := cmd.Flags().GetBool("architecture"); v {
			fmt.Println(PKG.Ctrl().Arch)
			return
		}

		if v, _ := cmd.Flags().GetBool("depends"); v {
			fmt.Println(strings.Join(PKG.Ctrl().Depends, ", "))
			return
		}

		if v, _ := cmd.Flags().GetBool("maintainer"); v {
			fmt.Println(PKG.Ctrl().Maintainer)
			return
		}

		if v, _ := cmd.Flags().GetBool("homepage"); v {
			fmt.Println(PKG.Ctrl().Homepage)
			return
		}

		fmt.Println(PKG.Ctrl().String())
	},
}
