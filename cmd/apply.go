package cmd

import (
	"github.com/deltron-fr/govisor/internal/process"
	"github.com/spf13/cobra"
)

var file string

var s = process.NewSupervisor()

var applyCmd = &cobra.Command{
	Use:     "apply",
	Aliases: []string{"a"},
	Short:   "Apply the process configuration",
	Long:    "Apply the process configuration defined in the YAML file to start and manage the processes.",
	RunE: func(cmd *cobra.Command, args []string) error {
		err := s.StartSupervisor(file)
		if err != nil {
			return err
		}
		return nil
	},
}

func init() {
	applyCmd.Flags().StringVarP(&file, "file", "f", "", "path to config file")
	applyCmd.MarkFlagRequired("file")

	rootCmd.AddCommand(applyCmd)
}
