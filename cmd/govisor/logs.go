package main

import "github.com/spf13/cobra"

var logsCmd = &cobra.Command{
	Use:     "logs [process_name]",
	Aliases: []string{"l"},
	Short:   "Retrieve the logs for a specific process",
	Long:    "Retrieve the logs for a specific process",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		err := appClient.LogsHandler(cmd.OutOrStderr(), args[0])
		if err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(logsCmd)
}
