package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/edmonl/opqu-sandbox/internal/sandbox"
	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete [name]",
	Short: "Remove a sandbox, its base tarball, and network interfaces",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if err := sandbox.ValidateName(name); err != nil {
			return err
		}

		running, err := sandbox.IsRunning(name)
		if err != nil {
			return err
		}

		if running {
			return errors.New("cannot delete a running sandbox")
		}

		if err := sudo(); err != nil {
			return err
		}

		rootfs := filepath.Join(rootDir, "rootfs")
		if err := os.RemoveAll(filepath.Join(rootfs, name)); err != nil {
			return fmt.Errorf("failed to delete sandbox rootfs: %v", err)
		}

		tarball := filepath.Join(rootfs, fmt.Sprintf("%v.base.tar.zst", name))
		if err := os.Remove(tarball); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("failed to delete base tarball: %v", err)
		}

		confDir := filepath.Join(rootDir, "conf")
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
