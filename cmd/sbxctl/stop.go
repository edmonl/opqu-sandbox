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
		if err := sudo(); err != nil {
			return err
		}
		name := args[0]
		if err := sandbox.ValidateName(name); err != nil {
			return err
		}

		running, err := sandbox.IsRunning(name)
		if err != nil {
			return err
		}

		if !running {
			return nil // Already stopped
		}

		machine := sandbox.MachineName(name)
		return sandbox.RunCmd("machinectl", "poweroff", machine)
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
