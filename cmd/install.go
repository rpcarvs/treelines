package cmd

import "lines/cmd/installskills"

func init() {
	rootCmd.AddCommand(installskills.NewCommand())
}
