package main

import "github.com/spf13/cobra"

var startCommand = &cobra.Command{
	Use:   "start",
	Short: "Start the govisor server",
	Long:  "Start the govisor server",
	RunE: func(cmd *cobra.Command, args []string) error {
		appClient.StartHandler(cmd.OutOrStderr())
		return nil
	},
}

func init() {
	rootCmd.AddCommand(startCommand)
}
