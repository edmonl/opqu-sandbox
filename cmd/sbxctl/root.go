package main

import (
	"fmt"
	"os"
	"path/filepath"

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
