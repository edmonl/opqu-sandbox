package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/edmonl/opqu-sandbox/internal/sandbox"
	"github.com/spf13/cobra"
)

var restoreCmd = &cobra.Command{
	Use:   "restore [name] [snapshot_path]",
	Short: "Replace a sandbox rootfs from a snapshot archive",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		snapshotPath := args[1]

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

		info, err := os.Stat(snapshotPath)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("snapshot '%s' does not exist", snapshotPath)
			}
			return fmt.Errorf("failed to stat snapshot '%s': %v", snapshotPath, err)
		}

		if !info.Mode().IsRegular() {
			return fmt.Errorf("snapshot '%s' is not a regular file", snapshotPath)
		}

		// Check snapshot contents
		expectedRootfs := fmt.Sprintf("rootfs-%s/", name)
		tarCmd := exec.Command("tar", "--zstd", "-tf", snapshotPath)
		output, err := tarCmd.Output()
		if err != nil {
			return fmt.Errorf("failed to list snapshot contents: %v", err)
		}

		foundRootfs := false
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) == expectedRootfs {
				foundRootfs = true
				break
			}
		}

		if !foundRootfs {
			return fmt.Errorf("snapshot '%s' does not contain top-level '%s'", snapshotPath, expectedRootfs)
		}

		rootfs := filepath.Join(rootDir, "rootfs-"+name)
		if err := os.RemoveAll(rootfs); err != nil {
			return fmt.Errorf("failed to remove rootfs for sandbox '%s': %v", name, err)
		}

		extractCmd := exec.Command("tar", "--zstd", "-xf", snapshotPath, "-C", rootDir)
		extractCmd.Stdout = os.Stdout
		extractCmd.Stderr = os.Stderr

		if err := extractCmd.Run(); err != nil {
			return fmt.Errorf("failed to extract snapshot: %v", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(restoreCmd)
}
