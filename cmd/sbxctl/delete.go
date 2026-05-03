package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/edmonl/opqu-sandbox/internal/sandbox"
	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete [name]",
	Short: "Remove a sandbox",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if err := sandbox.EnsureStopped(name); err != nil {
			return err
		}

		if err := sudo(); err != nil {
			return err
		}

		if err := sandbox.RemoveRootfs(rootDir, name); err != nil {
			return fmt.Errorf("failed to delete sandbox rootfs: %v", err)
		}

		if err := os.RemoveAll(sandbox.BaseTarballPath(rootDir, name)); err != nil {
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
