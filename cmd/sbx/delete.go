package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/edmonl/opqu-sandbox/internal/config"
	"github.com/edmonl/opqu-sandbox/internal/sandbox"
	"github.com/edmonl/opqu-sandbox/internal/util"
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

		// The managed rootfs must be a real directory and inactive.
		rootfsPath := filepath.Join(sbxDir, "rootfs", name)
		rootfsExists, err := sandbox.RequireInactiveRootfs(rootfsPath)
		if err != nil {
			return err
		}

		// The image entry for machinectl must be symlink pointing to the managed rootfs.
		imageSymlink := filepath.Join(conf.ImagesPath, name)
		if ok, err := util.CheckSymlinkTarget(imageSymlink, rootfsPath); err != nil {
			return fmt.Errorf("invalid sandbox image symlink: %w", err)
		} else if !ok {
			return fmt.Errorf("sandbox image symlink %v does not point to rootfs %v", imageSymlink, rootfsPath)
		}

		// Ask machinectl to remove image if the rootfs is present.
		// Dead symlinks are cleaned up locally instead.
		if rootfsExists {
			if err := util.RunCmd("machinectl", "remove", name); err != nil {
				return fmt.Errorf("failed to remove sandbox image %v using machinectl: %w", name, err)
			}
		}

		// Remove the image symlink.
		if err := os.RemoveAll(imageSymlink); err != nil {
			return fmt.Errorf("failed to delete sandbox image symlink %v: %w", imageSymlink, err)
		}

		// The nspawn symlink is best-effort cleanup.
		// Leave it alone if it was repointed manually, but keep deleting the file owned by this sandbox.
		nspawnFile := filepath.Join(sbxDir, "rootfs", name+".nspawn")
		nspawnSymlinkPath := filepath.Join(conf.NspawnFilesPath, name+".nspawn")
		if ok, err := util.CheckSymlinkTarget(nspawnSymlinkPath, nspawnFile); err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				util.Warn("failed to clean up nspawn file symlink: %v", err)
			}
		} else if !ok {
			util.Warn("keep nspawn file symlink %v because it does not point to %v", nspawnSymlinkPath, nspawnFile)
		} else if err := os.RemoveAll(nspawnSymlinkPath); err != nil {
			util.Warn("failed to delete nspawn file symlink %v: %v", nspawnSymlinkPath, err)
		}

		// Remove the managed nspawn file.
		if err := os.RemoveAll(nspawnFile); err != nil {
			util.Warn("failed to delete nspawn file %v: %v", nspawnFile, err)
		}

		// Snapshots are disposable output of the sandbox. Failure should not hide a successful rootfs cleanup.
		snapshotsDir := filepath.Join(sbxDir, "snapshots", name)
		if err := os.RemoveAll(snapshotsDir); err != nil {
			util.Warn("failed to delete snapshots directory %v: %v", snapshotsDir, err)
		}

		// Remove the managed rootfs last.
		if err := os.RemoveAll(rootfsPath); err != nil {
			return fmt.Errorf("failed to delete sandbox rootfs %v: %w", rootfsPath, err)
		}

		// User-managed configuration is intentionally preserved.
		// Report files that may still affect a future sandbox with the same name.
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
