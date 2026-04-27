package cmd

import (
	"fmt"
	"os"
	"os/exec"

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

		conf, err := config.Load(rootDir, name)
		if err != nil {
			return err
		}

		machine := sandbox.MachineName(name)
		userAtMachine := fmt.Sprintf("%s@%s", conf.SandboxUser, machine)

		execArgs := []string{"shell", userAtMachine}
		if len(args) > 1 {
			execArgs = append(execArgs, args[1:]...)
		}

		execCmd := exec.Command("machinectl", execArgs...)
		execCmd.Stdout = os.Stdout
		execCmd.Stderr = os.Stderr
		execCmd.Stdin = os.Stdin

		return execCmd.Run()
	},
}

func init() {
	rootCmd.AddCommand(shellCmd)
}
