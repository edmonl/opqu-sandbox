package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/edmonl/opqu-sandbox/internal/config"
	"github.com/edmonl/opqu-sandbox/internal/sandbox"
	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete [name]",
	Short: "Delete a sandbox",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if err := sandbox.EnsureStopped(name); err != nil {
			return err
		}

		if err := sandbox.Sudo(sbxDir); err != nil {
			return err
		}

		conf, err := config.LoadConf(sbxDir, name)
		if err != nil {
			return err
		}

		if err := sandbox.RunCmd("machinectl", "remove", name); err != nil {
			return fmt.Errorf("failed to delete sandbox rootfs %v using machinectl: %w", name, err)
		}

		nspawnFile := filepath.Join(conf.ImagePath, name+".nspawn")
		if err := os.Remove(nspawnFile); err != nil && !errors.Is(err, fs.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "Warning: failed to delete nspawn config file %v: %v\n", nspawnFile, err)
		}

		snapshotsDir := filepath.Join(sbxDir, "snapshots", name)
		if err := os.RemoveAll(snapshotsDir); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to delete snapshots directory %v: %v\n", snapshotsDir, err)
		}

		confDir := filepath.Join(sbxDir, "conf")
		var found []string
		for _, ext := range []string{".conf", ".packages", ".mounts"} {
			fName := name + ext
			if _, err := os.Stat(filepath.Join(confDir, fName)); err == nil {
				found = append(found, fName)
			}
		}

		if len(found) > 0 {
			fmt.Println("Keeping configuration files:")
			for _, f := range found {
				fmt.Println(f)
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)
}
