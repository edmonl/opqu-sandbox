package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"

	"github.com/edmonl/opqu-sandbox/internal/util"
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

		if err := ensureSbxDir(sbxDir); err != nil {
			return err
		}

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func ensureSbxDir(path string) error {
	if info, err := os.Stat(path); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("sandbox directory %v is not a directory", path)
		}
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("failed to access sandbox directory %v: %w", path, err)
	}

	if info, err := os.Stat(filepath.Dir(path)); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("failed to create sandbox directory %v: parent directory does not exist", path)
		}
		return fmt.Errorf("failed to access parent of sandbox directory %v: %w", path, err)
	} else if !info.IsDir() {
		return fmt.Errorf("failed to create sandbox directory %v: parent path is not a directory", path)
	}

	if err := os.Mkdir(path, 0o755); err != nil {
		return fmt.Errorf("failed to create sandbox directory %v: %w", path, err)
	}
	invokingUser, err := util.InvokingUser()
	if err != nil {
		return err
	}
	if os.Geteuid() != invokingUser.UID {
		if err := util.ChownToUser(path, invokingUser); err != nil {
			return err
		}
	}
	return nil
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
