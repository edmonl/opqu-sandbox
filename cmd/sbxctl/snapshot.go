package main

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"

	"github.com/edmonl/opqu-sandbox/internal/sandbox"
	"github.com/klauspost/compress/zstd"
	"github.com/spf13/cobra"
)

var snapshotCmd = &cobra.Command{
	Use:   "snapshot [name] [output path]",
	Short: "Save a sandbox rootfs to a tar.zst archive",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if err := sandbox.EnsureStopped(name); err != nil {
			return err
		}

		if err := sudo(); err != nil {
			return err
		}

		var outputPath string
		if len(args) == 2 {
			outputPath = args[1]
		} else {
			cwd, _ := os.Getwd()
			outputPath = filepath.Join(cwd, fmt.Sprintf("%v.snapshot.tar.zst", sandbox.MachineName(name)))
		}

		if _, err := os.Stat(outputPath); err == nil {
			return errors.New("snapshot output already exists")
		}

		// Restore ownership to the original user if we are running under sudo/su
		uid := -1
		gid := -1
		if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
			if u, err := user.Lookup(sudoUser); err == nil {
				nu, uidErr := strconv.Atoi(u.Uid)
				ng, gidErr := strconv.Atoi(u.Gid)
				if uidErr == nil && gidErr == nil {
					uid = nu
					gid = ng
				}
			} else {
				return fmt.Errorf("failed to look up user %v: %w", sudoUser, err)
			}
		}

		if err := sandbox.Compress(sandbox.RootfsPath(rootDir, name), outputPath, zstd.SpeedBestCompression); err != nil {
			return fmt.Errorf("failed to create snapshot: %v", err)
		}

		if uid > 0 && gid > 0 {
			if err := os.Chown(outputPath, uid, gid); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to change ownership of %v: %v\n", outputPath, err)
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(snapshotCmd)
}
