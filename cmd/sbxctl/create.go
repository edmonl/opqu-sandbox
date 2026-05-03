package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/edmonl/opqu-sandbox/internal/config"
	"github.com/edmonl/opqu-sandbox/internal/sandbox"
	"github.com/edmonl/opqu-sandbox/internal/util"
	"github.com/klauspost/compress/zstd"
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

		rootfs := filepath.Join(rootDir, "rootfs")
		pkgCache := filepath.Join(rootDir, "pkg-cache")
		// best effort for the current user
		os.MkdirAll(rootfs, 0o755)
		os.MkdirAll(pkgCache, 0o755)

		if err := sudo(); err != nil {
			return err
		}

		if err := os.MkdirAll(rootfs, 0o755); err != nil {
			return fmt.Errorf("failed to create rootfs directory: %v", err)
		}
		if err := os.MkdirAll(pkgCache, 0o755); err != nil {
			return fmt.Errorf("failed to create pkg-cache directory: %v", err)
		}

		conf, err := config.LoadConf(rootDir, name)
		if err != nil {
			return err
		}

		packages, err := config.LoadPackages(rootDir, name)
		if err != nil {
			return err
		}

		sandboxFs := sandbox.RootfsPath(rootDir, name)
		if _, err := os.Stat(sandboxFs); err == nil {
			return errors.New("sandbox rootfs already exists")
		}

		tarball := sandbox.BaseTarballPath(rootDir, name)
		if _, err := os.Stat(tarball); err == nil {
			prompt := fmt.Sprintf("Base image %v already exists. Press [Enter] directly to recreate rootfs from the base image, or enter \"overwrite\" to overwrite it with a new rootfs (Ctrl+C to cancel): ", filepath.Base(tarball))
			input, err := util.Confirm(prompt)
			if err != nil {
				return err
			}
			if input == "" {
				if err := sandbox.Extract(tarball, rootfs); err != nil {
					return fmt.Errorf("failed to extract from base image: %v", err)
				}
				fmt.Printf("Sandbox %v recreated from existing base image.\n", name)
				return nil
			}
			if input != "overwrite" {
				fmt.Fprintf(os.Stderr, "Operation cancelled.\n")
				return nil
			}

			fmt.Println("Creating fresh rootfs and overwriting existing base image")
		}

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
				return fmt.Errorf("provisioning sandbox with mmdebstrap failed: %v", err)
			}
		} else if _, err := exec.LookPath("debootstrap"); err == nil {
			fmt.Println("mmdebstrap not found, falling back to debootstrap")

			debootstrapArgs := []string{
				"--variant=" + conf.Variant,
			}

			if len(packages) > 0 {
				debootstrapArgs = append(debootstrapArgs, "--include="+strings.Join(packages, ","))
			}

			debootstrapArgs = append(debootstrapArgs, conf.Distro, sandboxFs, conf.Mirror)

			if err := sandbox.RunCmd("debootstrap", debootstrapArgs...); err != nil {
				return fmt.Errorf("provisioning sandbox with debootstrap failed: %v", err)
			}

			// Provision sandbox: hostname, hosts, and user
			if err := sandbox.RunCmd("chroot", sandboxFs, "/bin/sh", "-c", getSetupScript(name, conf)); err != nil {
				return fmt.Errorf("failed to provision sandbox: %v", err)
			}

			// Enable networking
			if err := sandbox.RunCmd("chroot", sandboxFs, "systemctl", "enable", "systemd-networkd"); err != nil {
				return fmt.Errorf("failed to enable systemd-networkd: %v", err)
			}
		} else {
			return fmt.Errorf("neither mmdebstrap nor debootstrap found in PATH")
		}

		if err := sandbox.Compress(sandboxFs, tarball, zstd.SpeedDefault); err != nil {
			return fmt.Errorf("failed to create base tarball for sandbox: %v", err)
		}

		return nil
	},
}

func getSetupScript(name string, conf *config.Config) string {
	var rootAction string
	if conf.RootPassword == "" {
		rootAction = "passwd -l root"
	} else {
		rootAction = fmt.Sprintf("echo 'root:%v' | chpasswd", conf.RootPassword)
	}

	uid := conf.SandboxUser.Uid
	userName := conf.SandboxUser.Username
	var createUserCmd string
	if uid == "0" {
		createUserCmd = rootAction
	} else {
		createUserCmd = fmt.Sprintf("useradd -m -u %v -s /bin/bash %v && %v && passwd -l %v", uid, userName, rootAction, userName)
	}

	// Set hostname and basic /etc/hosts
	hostnameCmd := fmt.Sprintf("echo %v > /etc/hostname", name)
	hostsCmd := fmt.Sprintf("printf '127.0.0.1\\tlocalhost\\n127.0.1.1\\t%v\\n\\n# The following lines are desirable for IPv6 capable hosts\\n::1\\tlocalhost ip6-localhost ip6-loopback\\nff02::1\\tip6-allnodes\\nff02::2\\tip6-allrouters\\n' > /etc/hosts", name)

	return fmt.Sprintf("%v && %v && %v", hostnameCmd, hostsCmd, createUserCmd)
}

func init() {
	rootCmd.AddCommand(createCmd)
}
