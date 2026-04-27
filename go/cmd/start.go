package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

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

		rootfs := filepath.Join(rootDir, "rootfs-"+name)
		if _, err := os.Stat(rootfs); err != nil {
			return fmt.Errorf("sandbox '%s' does not exist", name)
		}

		conf, err := config.Load(rootDir, name)
		if err != nil {
			return err
		}

		machine := sandbox.MachineName(name)
		zone := sandbox.ZoneName(name)

		runArgs := []string{
			"--unit=" + machine,
			"--description=opqu-sandbox " + name,
			"--collect",
			"systemd-nspawn",
			"--boot",
			"--machine=" + machine,
			"--directory=" + rootfs,
			"--network-zone=" + zone,
			"--resolv-conf=" + conf.ResolvConf,
		}

		for _, m := range conf.Mounts {
			if m.ReadOnly {
				runArgs = append(runArgs, fmt.Sprintf("--bind-ro=%s:%s", m.HostPath, m.SandboxPath))
			} else {
				runArgs = append(runArgs, fmt.Sprintf("--bind=%s:%s", m.HostPath, m.SandboxPath))
			}
		}

		if conf.Ports != "" {
			ports := strings.Fields(conf.Ports)
			for _, p := range ports {
				runArgs = append(runArgs, "--port="+p)
			}
		}

		if conf.Audio {
			u, err := user.Current() // this will be root because of sudo escalation, wait...
			if err != nil {
				return err
			}
			// Wait, the bash script gets the host user's ID. If we run via sudo, id -u is 0.
			// Bash script ran build_audio_flags *before* sudo, or it didn't escalate inside the script.
			// The Bash script says "Most commands use sudo".
			// Ah, the bash script expects you to call it without sudo and it has `sudo systemd-run ...` inside.
			// So `id -u` inside the script gives the non-root user.
			// Since our Go app escalates via sudo, `os.Getuid()` will be 0. We need to look up SUDO_UID.

			uid := os.Getenv("SUDO_UID")
			if uid == "" {
				uid = u.Uid // Fallback if somehow not run via sudo but as root
			}

			runArgs = append(runArgs, fmt.Sprintf("--bind=/run/user/%s/pipewire-0:/run/user/host/pipewire-0", uid))
			runArgs = append(runArgs, "--setenv=PIPEWIRE_REMOTE=/run/user/host/pipewire-0")
		}

		execCmd := exec.Command("systemd-run", runArgs...)
		execCmd.Stdout = os.Stdout
		execCmd.Stderr = os.Stderr
		execCmd.Stdin = os.Stdin

		if err := execCmd.Run(); err != nil {
			return fmt.Errorf("failed to start sandbox '%s': %v", name, err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}
