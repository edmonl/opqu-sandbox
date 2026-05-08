package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/edmonl/opqu-sandbox/internal/config"
	"github.com/edmonl/opqu-sandbox/internal/sandbox"
	"github.com/edmonl/opqu-sandbox/internal/util"
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create rootfs for a new sandbox and save its clean base image",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if err := sandbox.ValidateName(name); err != nil {
			return err
		}

		// create those dirs with current user
		snapshotsDir := filepath.Join(sbxDir, "snapshots", name)
		if err := os.MkdirAll(snapshotsDir, 0o755); err != nil {
			return fmt.Errorf("failed to create snapshots directory %v: %w", snapshotsDir, err)
		}
		pkgCache := filepath.Join(sbxDir, "pkg-cache")
		if err := os.MkdirAll(pkgCache, 0o755); err != nil {
			return fmt.Errorf("failed to create pkg-cache directory %v: %w", pkgCache, err)
		}

		if err := sandbox.Sudo(sbxDir); err != nil {
			return err
		}

		conf, err := config.LoadConf(sbxDir, name)
		if err != nil {
			return err
		}

		packages, err := config.LoadPackages(sbxDir, name)
		if err != nil {
			return err
		}

		sandboxFs := filepath.Join(conf.ImagePath, name)
		if _, err := os.Stat(sandboxFs); err == nil {
			return fmt.Errorf("sandbox rootfs %v already exists", sandboxFs)
		}
		if err := os.MkdirAll(sandboxFs, 0o755); err != nil {
			return fmt.Errorf("failed to create rootfs directory %v: %w", sandboxFs, err)
		}

		var provisionSuccess bool
		defer func() {
			if !provisionSuccess {
				os.RemoveAll(sandboxFs)
			}
		}()

		if _, err := exec.LookPath("mmdebstrap"); err == nil {
			mmdebstrapArgs := []string{
				"--variant=" + conf.Variant,
				"--skip=essential/unlink",
				"--setup-hook=mkdir -p \"$1/var/cache/apt/archives/\"",
				"--setup-hook=" + fmt.Sprintf("sync-in %q /var/cache/apt/archives/", pkgCache),
				"--customize-hook=" + fmt.Sprintf("sync-out /var/cache/apt/archives %q", pkgCache),
				"--customize-hook=" + fmt.Sprintf(`chroot "$1" /bin/sh -c %q`, getSetupScript(name, conf)),
				"--customize-hook=" + `chroot "$1" systemctl enable systemd-networkd`,
			}

			if len(packages) > 0 {
				mmdebstrapArgs = append(mmdebstrapArgs, "--include="+strings.Join(packages, ","))
			}

			mmdebstrapArgs = append(mmdebstrapArgs, conf.Distro, sandboxFs, conf.Mirror)

			if err := sandbox.RunCmd("mmdebstrap", mmdebstrapArgs...); err != nil {
				return fmt.Errorf("provisioning sandbox %v with mmdebstrap failed: %w", name, err)
			}
		} else if _, err := exec.LookPath("debootstrap"); err == nil {
			debootstrapArgs := []string{
				"--variant=" + conf.Variant,
			}

			if len(packages) > 0 {
				debootstrapArgs = append(debootstrapArgs, "--include="+strings.Join(packages, ","))
			}

			debootstrapArgs = append(debootstrapArgs, conf.Distro, sandboxFs, conf.Mirror)

			if err := sandbox.RunCmd("debootstrap", debootstrapArgs...); err != nil {
				return fmt.Errorf("provisioning sandbox %v with debootstrap failed: %w", name, err)
			}

			// Provision sandbox: hostname, hosts, and user
			if err := sandbox.RunCmd("chroot", sandboxFs, "/bin/sh", "-c", getSetupScript(name, conf)); err != nil {
				return fmt.Errorf("failed to provision sandbox: %w", err)
			}

			// Enable networking
			if err := sandbox.RunCmd("chroot", sandboxFs, "systemctl", "enable", "systemd-networkd"); err != nil {
				return fmt.Errorf("failed to enable systemd-networkd: %w", err)
			}
		} else {
			return fmt.Errorf("neither mmdebstrap nor debootstrap found in PATH")
		}

		provisionSuccess = true

		if err := sandbox.CreateSnapshot(sandboxFs, snapshotsDir, "base"); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to create base snapshot: %v", err)
		}

		fmt.Printf("Successfully created sandbox %v\n", name)
		return nil
	},
}

func getSetupScript(name string, conf *config.Config) string {
	var rootAction string
	if conf.RootPassword == "" {
		rootAction = "passwd -l root"
	} else {
		rootAction = fmt.Sprintf("echo 'root:'%v | chpasswd", util.EscapeShellArg(conf.RootPassword))
	}

	uid := conf.SandboxUser.Uid
	userName := conf.SandboxUser.Username
	var createUserCmd string
	if uid == "0" {
		createUserCmd = rootAction
	} else {
		escapedUserName := util.EscapeShellArg(userName)
		createUserCmd = fmt.Sprintf("useradd -m -u %v -s /bin/bash %v && %v && passwd -l %v", uid, escapedUserName, rootAction, escapedUserName)
	}

	// Set hostname and basic /etc/hosts
	hostnameCmd := fmt.Sprintf("echo %v > /etc/hostname", util.EscapeShellArg(name))
	hostsCmd := fmt.Sprintf("printf '127.0.0.1\\tlocalhost\\n127.0.1.1\\t%%s\\n\\n# The following lines are desirable for IPv6 capable hosts\\n::1\\tlocalhost ip6-localhost ip6-loopback\\nff02::1\\tip6-allnodes\\nff02::2\\tip6-allrouters\\n' %v > /etc/hosts", util.EscapeShellArg(name))

	return fmt.Sprintf("%v && %v && %v", hostnameCmd, hostsCmd, createUserCmd)
}

func init() {
	rootCmd.AddCommand(createCmd)
}
