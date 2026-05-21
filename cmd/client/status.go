package main

import "github.com/spf13/cobra"

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Get the status of the processes",
	// TODO: change this because resource usage is not implemented yet
	Long: "Get the status of the processes managed by govisor, including their current state, uptime, and resource usage*.",
	RunE: func(cmd *cobra.Command, args []string) error {
		err := appClient.StatusHandler(cmd.OutOrStdout())
		if err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
