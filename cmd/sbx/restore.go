package main

import (
	"path/filepath"

	"github.com/edmonl/opqu-sandbox/internal/config"
	"github.com/edmonl/opqu-sandbox/internal/sandbox"
	"github.com/spf13/cobra"
)

var restoreCmd = &cobra.Command{
	Use:   "restore [name] [snapshot path]",
	Short: "Replace a sandbox rootfs from a snapshot archive",
	Args:  cobra.ExactArgs(2),
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

		snapshotPath := args[1]
		rootfsPath := filepath.Join(conf.ImagePath, name)
		return sandbox.ReplaceRootfs(rootfsPath, snapshotPath)
	},
}

func init() {
	rootCmd.AddCommand(restoreCmd)
}
