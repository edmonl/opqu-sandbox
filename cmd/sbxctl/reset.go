package main

import (
	"fmt"
	"os"
	"os/exec"
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

		conf, err := config.LoadConf(rootDir, name)
		if err != nil {
			return err
		}

		rootfs := filepath.Join(rootDir, "rootfs", name)
		if err := os.RemoveAll(rootfs); err != nil {
			return fmt.Errorf("failed to remove rootfs for sandbox %v: %v", name, err)
		}

		tarball := filepath.Join(rootDir, "rootfs", fmt.Sprintf("%v.base.tar.zst", name))
		if err := sandbox.Extract(tarball, filepath.Join(rootDir, "rootfs")); err != nil {
			return fmt.Errorf("failed to extract base tarball: %v", err)
		}

		bridge := sandbox.BridgeName(conf.NetworkZone)
		machine := sandbox.MachineName(name)

		// Ignore errors for cleanup commands
		exec.Command("ip", "link", "delete", bridge).Run()
		exec.Command("systemctl", "reset-failed", machine).Run()

		return nil
	},
}

func init() {
	rootCmd.AddCommand(resetCmd)
}
