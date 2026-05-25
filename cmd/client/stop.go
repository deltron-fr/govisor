package main

import "github.com/spf13/cobra"

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the running processes",
	Long:  "Stop the running processes managed by govisor",
	RunE: func(cmd *cobra.Command, args []string) error {
		err := appClient.StopHandler(cmd.OutOrStdout())
		if err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
