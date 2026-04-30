package main

import (
	"fmt"

	"github.com/edmonl/opqu-sandbox/internal/config"
	"github.com/edmonl/opqu-sandbox/internal/sandbox"
	"github.com/spf13/cobra"
)

var shellCmd = &cobra.Command{
	Use:   "shell [name] [command...]",
	Short: "Open a shell or run a command inside a running sandbox",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := sudo(); err != nil {
			return err
		}
		name := args[0]
		if err := sandbox.ValidateName(name); err != nil {
			return err
		}

		conf, err := config.LoadConf(rootDir, name)
		if err != nil {
			return err
		}

		machine := sandbox.MachineName(name)
		userAtMachine := fmt.Sprintf("%v@%v", conf.SandboxUser.Username, machine)

		execArgs := []string{"shell", userAtMachine}
		if len(args) > 1 {
			execArgs = append(execArgs, args[1:]...)
		}

		return sandbox.RunCmd("machinectl", execArgs...)
	},
}

func init() {
	rootCmd.AddCommand(shellCmd)
}
