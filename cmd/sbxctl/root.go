package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/spf13/cobra"
)

var rootDir string
var whitespacePattern = regexp.MustCompile(`\s`)

var rootCmd = &cobra.Command{
	Use:   "sbxctl",
	Short: "the single executable binary of opqu-sandbox",
	Long:  "A tool for managing disposable systemd-nspawn sandboxes.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if !cmd.HasParent() {
			return nil
		}

		if abs, err := filepath.Abs(rootDir); err == nil {
			if whitespacePattern.MatchString(rootDir) {
				return errors.New("sandbox root directory path cannot contain whitespace")
			}
			rootDir = abs
			// Create the root directory with the current user.
			// Ignore the error as the directory may be created later with root user.
			os.MkdirAll(rootDir, 0755)
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
