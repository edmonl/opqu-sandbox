package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/edmonl/opqu-sandbox/internal/sandbox"
	"github.com/klauspost/compress/zstd"
	"github.com/spf13/cobra"
)

var snapshotCmd = &cobra.Command{
	Use:   "snapshot [name] [output_path]",
	Short: "Save a stopped sandbox rootfs to a tar.zst archive",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := sudo(); err != nil {
			return err
		}
		name := args[0]
		if err := sandbox.ValidateName(name); err != nil {
			return err
		}

		running, err := sandbox.IsRunning(name)
		if err != nil {
			return err
		}

		if running {
			return fmt.Errorf("sandbox %v is running; stop it first", name)
		}

		var outputPath string
		if len(args) == 2 {
			outputPath = args[1]
		} else {
			cwd, _ := os.Getwd()
			outputPath = filepath.Join(cwd, fmt.Sprintf("opqu-sbx-%v.snapshot.tar.zst", name))
		}

		if _, err := os.Stat(outputPath); err == nil {
			return fmt.Errorf("snapshot output %v already exists; move it away first", outputPath)
		}

		if err := sandbox.Compress(filepath.Join(rootDir, "rootfs", name), outputPath, zstd.SpeedBestCompression); err != nil {
			return fmt.Errorf("failed to create snapshot: %v", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(snapshotCmd)
}
