package main

import (
	"fmt"

	"github.com/penguinpowernz/go-ian/util/tell"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func init() {
	addFieldFlags(setCmd)
	rootCmd.AddCommand(setCmd)

}

var setCmd = &cobra.Command{
	Use:   "set",
	Short: "Set a value in the control file",
	Long:  `Allows to set a value in the control file via the command line`,
	Run: func(cmd *cobra.Command, args []string) {
		PKG = readPkg(DIR)

		if updatePkgFromFlags(cmd.Flags().Lookup) {
			err := PKG.Ctrl().WriteFile(PKG.CtrlFile())
			tell.IfFatalf(err, "couldn't set fields")
			return
		}

		tell.Fatalf("No fields given")
	},
}

func updatePkgFromFlags(flag func(string) *pflag.Flag) bool {
	var anySet bool
	if v := flag("arch").Value.String(); v != "" {
		PKG.Ctrl().Version = v
		fmt.Println("Architecture set to", v)
		anySet = true
	}

	if v := flag("version").Value.String(); v != "" {
		PKG.Ctrl().Version = v
		fmt.Println("Version set to", v)
		anySet = true
	}

	if v := flag("maintainer").Value.String(); v != "" {
		PKG.Ctrl().Maintainer = v
		fmt.Println("Maintainer set to", v)
		anySet = true
	}

	if v := flag("description").Value.String(); v != "" {
		PKG.Ctrl().Desc = v
		fmt.Println("Description set to", v)
		anySet = true
	}

	if v := flag("long-description").Value.String(); v != "" {
		PKG.Ctrl().LongDesc = v
		fmt.Println("Long Description set to", v)
		anySet = true
	}

	if v := flag("name").Value.String(); v != "" {
		PKG.Ctrl().Name = v
		fmt.Println("Name set to", v)
		anySet = true
	}

	return anySet
}

func addFieldFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("name", "n", "", "set the name")
	cmd.Flags().StringP("arch", "a", "", "set the architecture")
	cmd.Flags().StringP("version", "v", "", "set the version")
	cmd.Flags().StringP("maintainer", "m", "", "set the maintainer")
	cmd.Flags().StringP("description", "D", "", "set the description")
	cmd.Flags().StringP("long-description", "L", "", "set the long description")
}
