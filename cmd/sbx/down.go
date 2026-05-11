package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/edmonl/opqu-sandbox/internal/config"
	"github.com/edmonl/opqu-sandbox/internal/sandbox"
	"github.com/spf13/cobra"
)

var downCmd = &cobra.Command{
	Use:   "down [name]",
	Short: "Power off a running sandbox",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if err := sandbox.ValidateName(name); err != nil {
			return err
		}

		if err := sandbox.Sudo(sbxDir); err != nil {
			return err
		}
		
		if running, err := sandbox.IsRunning(name); err != nil {
			return err
		} else if !running {
			fmt.Fprintf(os.Stderr, "Warning: sandbox %v is not running\n", name)
		} else if err := sandbox.RunCmd("machinectl", "poweroff", name); err != nil {
			return fmt.Errorf("failed to power off sandbox %v: %w", name, err)
		}

		if err := sandbox.RunCmd("machinectl", "disable", name); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to disable sandbox %v: %v\n", name, err)
		}

		if conf, err := config.LoadConf(sbxDir, name); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to delete nspawn file: %v\n", err)
		} else {
			nspawnFile := filepath.Join(conf.ImagePath, name+".nspawn")
			if err := os.RemoveAll(nspawnFile); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to delete nspawn file %v: %v\n", nspawnFile, err)
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(downCmd)
}
