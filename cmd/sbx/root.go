package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/spf13/cobra"
)

var sbxDir string
var whitespacePattern = regexp.MustCompile(`\s`)

var rootCmd = &cobra.Command{
	Use:          "sbx",
	Short:        "the single executable binary of opqu-sandbox",
	Long:         "A tool for managing disposable systemd-nspawn sandboxes.",
	SilenceUsage: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if !cmd.HasParent() {
			return nil
		}

		if abs, err := filepath.Abs(sbxDir); err != nil {
			return fmt.Errorf("failed to resolve absolute path for %v: %w", sbxDir, err)
		} else {
			sbxDir = abs
		}

		if whitespacePattern.MatchString(sbxDir) {
			return fmt.Errorf("sandbox directory path %v cannot contain whitespace", sbxDir)
		}

		if err := os.MkdirAll(sbxDir, 0755); err != nil {
			return fmt.Errorf("failed to create sandbox directory %v: %w", sbxDir, err)
		}

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func init() {
	defaultRoot := os.Getenv("OPQU_SBX_DIRECTORY")
	if defaultRoot == "" {
		defaultRoot = "."
	}

	rootCmd.PersistentFlags().StringVarP(
		&sbxDir, "sbx-dir", "d", defaultRoot,
		"sandbox directory overriding env OPQU_SBX_DIRECTORY",
	)
}
