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
		name := args[0]
		if err := sandbox.ValidateName(name); err != nil {
			return err
		}

		conf, err := config.LoadConf(rootDir, name)
		if err != nil {
			return err
		}

		execArgs := []string{
			"shell",
			fmt.Sprintf("%v@%v", conf.SandboxUser.Username, sandbox.MachineName(name)),
		}
		if len(args) > 1 {
			execArgs = append(execArgs, args[1:]...)
		}

		return sandbox.RunCmd("machinectl", execArgs...)
	},
}

func init() {
	rootCmd.AddCommand(shellCmd)
}
