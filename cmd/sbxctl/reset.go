package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

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
			return fmt.Errorf("sandbox '%s' is running; stop it first", name)
		}

		rootfs := filepath.Join(rootDir, "rootfs", name)
		if err := os.RemoveAll(rootfs); err != nil {
			return fmt.Errorf("failed to remove rootfs for sandbox '%s': %v", name, err)
		}

		tarball := filepath.Join(rootDir, "rootfs", fmt.Sprintf("%s.base.tar.zst", name))
		tarCmd := exec.Command("tar", "--zstd", "-xf", tarball, "-C", filepath.Join(rootDir, "rootfs"))
		tarCmd.Stdout = os.Stdout
		tarCmd.Stderr = os.Stderr
		if err := tarCmd.Run(); err != nil {
			return fmt.Errorf("failed to extract base tarball: %v", err)
		}

		bridge := sandbox.BridgeName(name)
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
