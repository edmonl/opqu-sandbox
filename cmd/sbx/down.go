package main

import (
	"fmt"

	"github.com/edmonl/opqu-sandbox/internal/sandbox"
	"github.com/spf13/cobra"
)

var downCmd = &cobra.Command{
	Use:   "down [name]",
	Short: "Power off a running sandbox",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if err := sandbox.ValidateName(name); err != nil {
			return err
		}

		running, err := sandbox.IsRunning(name)
		if err != nil {
			return err
		}
		if !running {
			fmt.Println("Not running")
			return nil // Already stopped
		}

		return sandbox.RunCmd("machinectl", "poweroff", sandbox.MachineName(name))
	},
}

func init() {
	rootCmd.AddCommand(downCmd)
}
