package cmd

import "github.com/rpcarvs/treelines/cmd/installskills"

func init() {
	rootCmd.AddCommand(installskills.NewCommand())
}
