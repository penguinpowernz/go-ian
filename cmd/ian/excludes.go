package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(excludesCmd)
}

var excludesCmd = &cobra.Command{
	Use:   "excludes",
	Short: "List package excludes",
	Run: func(cmd *cobra.Command, args []string) {
		for _, s := range PKG.Excludes() {
			fmt.Println(s)
		}
	},
}
