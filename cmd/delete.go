package cmd

import (
	"fmt"
	"os"
	"os/exec"
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
			return fmt.Errorf("sandbox '%s' is running; stop it first", name)
		}

		rootfs := filepath.Join(rootDir, "rootfs-"+name)
		if err := os.RemoveAll(rootfs); err != nil {
			return fmt.Errorf("failed to remove rootfs for sandbox '%s': %v", name, err)
		}

		tarball := filepath.Join(rootDir, fmt.Sprintf("rootfs-%s.base.tar.zst", name))
		if err := os.Remove(tarball); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove base tarball for sandbox '%s': %v", name, err)
		}

		// Check for kept configs
		configFiles := []string{
			filepath.Join(rootDir, "conf", name+".conf"),
			filepath.Join(rootDir, "conf", name+".packages"),
			filepath.Join(rootDir, "conf", name+".mounts"),
		}

		var found []string
		for _, f := range configFiles {
			if _, err := os.Stat(f); err == nil {
				found = append(found, filepath.Base(f))
			}
		}

		if len(found) > 0 {
			fmt.Println("Keeping configuration files in conf/:")
			for _, f := range found {
				fmt.Printf("  %s\n", f)
			}
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
	rootCmd.AddCommand(deleteCmd)
}
