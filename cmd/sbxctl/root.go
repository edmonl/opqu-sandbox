package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var rootDir string

var rootCmd = &cobra.Command{
	Use:   "sbxctl",
	Short: "sbxctl is a tool for managing disposable Debian sandboxes",
	Long:  `A Go implementation of the opqu-sandbox management tool.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Only escalate if not already root
		if os.Geteuid() != 0 {
			// Skip escalation for help
			if cmd.Name() == "help" {
				return nil
			}

			fmt.Print("This operation requires root privileges. Press [Enter] to escalate, or any other key to cancel: ")
			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				return err
			}

			if strings.TrimSpace(input) != "" {
				fmt.Println("Escalation cancelled.")
				os.Exit(0)
			}

			exe, err := os.Executable()

			if err != nil {
				return err
			}

			// Re-run with sudo
			sudoArgs := append([]string{exe}, os.Args[1:]...)
			sudoCmd := exec.Command("sudo", sudoArgs...)
			sudoCmd.Stdin = os.Stdin
			sudoCmd.Stdout = os.Stdout
			sudoCmd.Stderr = os.Stderr

			return sudoCmd.Run()
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cwd, _ := os.Getwd()
	rootCmd.PersistentFlags().StringVar(&rootDir, "root", os.Getenv("OPQU_SBX_ROOT"), "sandbox root directory (default $PWD or $OPQU_SBX_ROOT)")

	if rootDir == "" {
		rootDir = cwd
	}

	// Resolve absolute path
	abs, err := filepath.Abs(rootDir)
	if err == nil {
		rootDir = abs
	}
}
