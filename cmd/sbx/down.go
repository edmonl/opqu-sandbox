package main

import (
	"fmt"

	"github.com/edmonl/opqu-sandbox/internal/config"
	"github.com/edmonl/opqu-sandbox/internal/sandbox"
	"github.com/edmonl/opqu-sandbox/internal/util"
	"github.com/spf13/cobra"
)

var downCmd = &cobra.Command{
	Use:     "down [name]",
	Aliases: []string{"d"},
	Short:   "Power off a running sandbox",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if err := sandbox.ValidateName(name); err != nil {
			return err
		}

		if err := sandbox.Sudo(sbxDir); err != nil {
			return err
		}

		if running, e := sandbox.IsRunning(name); e != nil {
			return e
		} else if !running {
			util.Warn("sandbox %v is not running", name)
		} else if e := util.RunCmd("machinectl", "poweroff", name); e != nil {
			return fmt.Errorf("failed to power off sandbox %v: %w", name, e)
		}

		if e := util.RunCmd("machinectl", "disable", name); e != nil {
			util.Warn("failed to disable sandbox %v: %v", name, e)
		}

		if conf, err := config.LoadConf(sbxDir, name); err != nil {
			util.Warn("failed to load configuration for nspawn file cleanup: %v", err)
		} else {
			sandbox.RemoveNspawnFile(sbxDir, name, conf)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(downCmd)
}
