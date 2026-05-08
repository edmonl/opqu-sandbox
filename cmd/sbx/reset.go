package main

import (
	"errors"
	"path/filepath"

	"github.com/edmonl/opqu-sandbox/internal/config"
	"github.com/edmonl/opqu-sandbox/internal/sandbox"
	"github.com/spf13/cobra"
)

var resetCmd = &cobra.Command{
	Use:   "reset [name]",
	Short: "Restore a sandbox from its clean base image",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if err := sandbox.EnsureStopped(name); err != nil {
			return err
		}

		conf, err := config.LoadConf(sbxDir, name)
		if err != nil {
			return err
		}

		if err := sandbox.Sudo(sbxDir); err != nil {
			return err
		}

		snapshotsDir := filepath.Join(sbxDir, "snapshots", name)
		var existingBase string
		matches, err := filepath.Glob(filepath.Join(snapshotsDir, "base.*.tar.zst"))
		if err == nil && len(matches) > 0 {
			existingBase = matches[0] // Use the first match
		}

		if existingBase == "" {
			return errors.New("base snapshot not found")
		}

		rootfsPath := filepath.Join(conf.ImagePath, name)
		return sandbox.ReplaceRootfs(rootfsPath, existingBase)
	},
}

func init() {
	rootCmd.AddCommand(resetCmd)
}
