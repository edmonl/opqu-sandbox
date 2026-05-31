package main

import (
	"path/filepath"

	"github.com/edmonl/opqu-sandbox/internal/config"
	"github.com/edmonl/opqu-sandbox/internal/sandbox"
	"github.com/spf13/cobra"
)

var snapshotCmd = &cobra.Command{
	Use:   "snapshot [name] [snapshot name]",
	Short: "Save a sandbox rootfs to a tar.zst archive",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if err := sandbox.EnsureStopped(name); err != nil {
			return err
		}

		snapshotName := args[1]
		if err := sandbox.ValidateSnapshotName(snapshotName); err != nil {
			return err
		}

		conf, err := config.LoadConf(sbxDir, name)
		if err != nil {
			return err
		}

		snapshotsDir, err := sandbox.MkdirAllAsUser(conf, sbxDir, "snapshots", name)
		if err != nil {
			return err
		}

		if err := sandbox.Sudo(sbxDir); err != nil {
			return err
		}

		if err := sandbox.CreateSnapshot(filepath.Join(sbxDir, "rootfs", name), snapshotsDir, snapshotName, conf.SandboxUser); err != nil {
			return err
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(snapshotCmd)
}
