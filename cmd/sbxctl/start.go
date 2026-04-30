package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/edmonl/opqu-sandbox/internal/config"
	"github.com/edmonl/opqu-sandbox/internal/sandbox"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start [name]",
	Short: "Boot a sandbox with configured mounts, ports, and optional audio",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if err := sandbox.ValidateName(name); err != nil {
			return err
		}

		if err := sudo(); err != nil {
			return err
		}

		rootfs := filepath.Join(rootDir, "rootfs", name)
		if _, err := os.Stat(rootfs); err != nil {
			return fmt.Errorf("sandbox %v does not exist", name)
		}

		conf, err := config.LoadConf(rootDir, name)
		if err != nil {
			return err
		}

		mounts, err := config.LoadMounts(rootDir, name, conf.SandboxUser)
		if err != nil {
			return err
		}

		machine := sandbox.MachineName(name)

		runArgs := []string{
			"--unit=" + machine,
			"--description=opqu-sandbox " + name,
			"--collect",
			"systemd-nspawn",
			"--boot",
			"--machine=" + machine,
			"--directory=" + rootfs,
			"--network-zone=" + conf.NetworkZone,
			"--resolv-conf=" + conf.ResolvConf,
		}

		for _, m := range mounts {
			hostPath := m.HostPath
			if hostPath == "" {
				tmp, err := os.MkdirTemp("/var/tmp", "sbx-scratch-"+name+"-")
				if err != nil {
					return fmt.Errorf("failed to create scratch directory: %v", err)
				}
				hostPath = tmp
				runArgs = append(runArgs, "--property=ExecStopPost=/bin/rm -rf "+hostPath)
			}

			if m.ReadOnly {
				runArgs = append(runArgs, fmt.Sprintf("--bind-ro=%v:%v", hostPath, m.SandboxPath))
			} else {
				runArgs = append(runArgs, fmt.Sprintf("--bind=%v:%v", hostPath, m.SandboxPath))
			}
		}

		for _, p := range conf.Ports {
			runArgs = append(runArgs, "--port="+p)
		}

		if err := sandbox.RunCmd("systemd-run", runArgs...); err != nil {
			return fmt.Errorf("failed to start sandbox %v: %v", name, err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}
