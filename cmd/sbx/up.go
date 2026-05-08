package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/edmonl/opqu-sandbox/internal/config"
	"github.com/edmonl/opqu-sandbox/internal/sandbox"
	"github.com/spf13/cobra"
)

var upCmd = &cobra.Command{
	Use:   "up [name]",
	Short: "Power on a sandbox",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if err := sandbox.ValidateName(name); err != nil {
			return err
		}

		if err := sandbox.Sudo(sbxDir); err != nil {
			return err
		}

		sandboxFs := filepath.Join(sbxDir, "rootfs", name)
		if _, err := os.Stat(sandboxFs); err != nil {
			return fmt.Errorf("cannot access sandbox rootfs: %w", err)
		}

		conf, err := config.LoadConf(sbxDir, name)
		if err != nil {
			return err
		}

		mounts, err := config.LoadMounts(sbxDir, name, conf.SandboxUser)
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
			"--directory=" + sandboxFs,
			"--network-zone=" + conf.NetworkZone,
			"--resolv-conf=" + conf.ResolvConf,
		}

		for _, m := range mounts {
			var flag string
			if m.ReadOnly {
				flag = "--bind-ro"
			} else {
				flag = "--bind"
			}

			runArgs = append(runArgs, fmt.Sprintf("%v=%v:%v", flag, m.HostPath, m.SandboxPath))
		}

		for _, p := range conf.Ports {
			runArgs = append(runArgs, "--port="+p)
		}

		if err := sandbox.RunCmd("systemd-run", runArgs...); err != nil {
			return fmt.Errorf("failed to start sandbox: %v", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(upCmd)
}
