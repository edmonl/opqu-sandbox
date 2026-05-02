package main

import (
	"github.com/edmonl/opqu-sandbox/internal/sandbox"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop [name]",
	Short: "Power off a running sandbox",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		running, err := sandbox.IsRunning(name)
		if err != nil {
			return err
		}

		if !running {
			return nil // Already stopped
		}

		return sandbox.RunCmd("machinectl", "poweroff", sandbox.MachineName(name))
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
