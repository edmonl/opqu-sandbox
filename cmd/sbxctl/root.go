package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var rootDir string

var rootCmd = &cobra.Command{
	Use:   "sbxctl",
	Short: "the single executable binary of opqu-sandbox",
	Long:  "A tool for managing disposable systemd-nspawn sandboxes.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if !cmd.HasParent() {
			return nil
		}

		if abs, err := filepath.Abs(rootDir); err == nil {
			rootDir = abs
		} else {
			return fmt.Errorf("failed to resolve absolute path for root: %w", err)
		}

		// Only escalate if not already root
		if os.Geteuid() != 0 {
			fmt.Print("This operation requires root privileges. Press [Enter] directly to escalate, or Ctrl+C to cancel: ")
			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err == io.EOF || strings.TrimSpace(input) != "" {
				if err == io.EOF { 
					fmt.Println()
				}
				fmt.Println("Escalation cancelled.")
				os.Exit(0)
			}
			if err != nil {
				return err
			}

			exe, err := os.Executable()
			if err != nil {
				return fmt.Errorf("failed to resolve the executable path that started the current process: %w", err)
			}

			// Re-run with sudo
			sudoCmd := exec.Command("sudo", append([]string{exe}, os.Args[1:]...)...)
			sudoCmd.Stdin = os.Stdin
			sudoCmd.Stdout = os.Stdout
			sudoCmd.Stderr = os.Stderr
			return sudoCmd.Run()
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func init() {
	defaultRoot := os.Getenv("OPQU_SBX_ROOT")
	if defaultRoot == "" {
		defaultRoot = "."
	}

	rootCmd.PersistentFlags().StringVar(
		&rootDir, "root", defaultRoot,
		"sandbox root directory overriding env OPQU_SBX_ROOT",
	)
}
