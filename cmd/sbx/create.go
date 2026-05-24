package main

import (
	"errors"
	"fmt"
	"io/fs"
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
	Short: "Create rootfs for a new sandbox",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if err := sandbox.ValidateName(name); err != nil {
			return err
		}

		// create those dirs with current user
		rootfsDir, err := createDir(sbxDir, "rootfs")
		if err != nil {
			return err
		}
		snapshotsDir, err := createDir(sbxDir, "snapshots", name)
		if err != nil {
			return err
		}
		pkgCache, err := createDir(sbxDir, "pkg-cache")
		if err != nil {
			return err
		}

		// sudo
		if e := sandbox.Sudo(sbxDir); e != nil {
			return e
		}

		// conf
		conf, err := config.LoadConf(sbxDir, name)
		if err != nil {
			return err
		}
		packages, err := config.LoadPackages(sbxDir, name)
		if err != nil {
			return err
		}

		// check
		if err := os.MkdirAll(conf.ImagesPath, 0o700); err != nil {
			return fmt.Errorf("failed to create images directory %v: %w", conf.ImagesPath, err)
		}

		sandboxFs := filepath.Join(rootfsDir, name)
		if err := os.Mkdir(sandboxFs, 0o755); err != nil {
			if errors.Is(err, fs.ErrExist) {
				return fmt.Errorf("sandbox rootfs %v already exists", sandboxFs)
			}
			return fmt.Errorf("failed to create sandbox rootfs directory %v: %w", sandboxFs, err)
		}

		var createSuccess bool
		defer func() {
			if createSuccess {
				return
			}

			hasMounts, err := sandbox.HasMounts(sandboxFs)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to clean up %v: %v\n", sandboxFs, err)
				return
			}
			if hasMounts {
				fmt.Fprintf(os.Stderr, "Warning: failed to clean up %v: active mounts detected\n", sandboxFs)
				return
			}
			if err := os.RemoveAll(sandboxFs); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to clean up %v: %v\n", sandboxFs, err)
			}
		}()

		if _, err := exec.LookPath("mmdebstrap"); err == nil {
			mmdebstrapArgs := []string{
				"--variant=" + conf.Variant,
				"--skip=essential/unlink",
				"--setup-hook=mkdir -p \"$1/var/cache/apt/archives/\"",
				"--setup-hook=" + fmt.Sprintf("sync-in %q /var/cache/apt/archives/", pkgCache),
				"--customize-hook=" + fmt.Sprintf("sync-out /var/cache/apt/archives %q", pkgCache),
				"--customize-hook=" + fmt.Sprintf(`chroot "$1" /bin/sh -c %q`, getSetupScript(conf)),
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

			// Provision sandbox user.
			if err := sandbox.RunCmd("chroot", sandboxFs, "/bin/sh", "-c", getSetupScript(conf)); err != nil {
				return fmt.Errorf("failed to provision sandbox: %w", err)
			}

			// Enable networking
			if err := sandbox.RunCmd("chroot", sandboxFs, "systemctl", "enable", "systemd-networkd"); err != nil {
				return fmt.Errorf("failed to enable systemd-networkd: %w", err)
			}
		} else {
			return fmt.Errorf("neither mmdebstrap nor debootstrap found in PATH")
		}

		if err := writeSandboxFiles(sandboxFs, name); err != nil {
			return err
		}

		// Clean up apt partial directory to prevent permission errors for unprivileged users
		if err := os.RemoveAll(filepath.Join(pkgCache, "partial")); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to delete pkg-cache/partial: %v\n", err)
		}

		if err := sandbox.CreateSymlink(sandboxFs, filepath.Join(conf.ImagesPath, name)); err != nil {
			return fmt.Errorf("failed to create sandbox image symlink: %w", err)
		}
		createSuccess = true

		if err := sandbox.CreateSnapshot(sandboxFs, snapshotsDir, "base"); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to create base snapshot: %v\n", err)
		}

		return nil
	},
}

func getSetupScript(conf *config.Config) string {
	commands := []string{"set -e"}

	uid := conf.SandboxUser.Uid
	if uid != "0" {
		gid := conf.SandboxUser.Gid
		escapedUserName := util.EscapeShellArg(conf.SandboxUser.Username)
		commands = append(commands,
			fmt.Sprintf("groupadd -g %v %v", gid, escapedUserName),
			fmt.Sprintf("useradd -m -u %v -g %v -s /bin/bash %v", uid, gid, escapedUserName),
			fmt.Sprintf("passwd -l %v", escapedUserName),
		)
	}

	if conf.RootPassword == "" {
		commands = append(commands, "passwd -l root")
	} else {
		commands = append(commands, fmt.Sprintf("printf '%%s\\n' 'root:'%v | chpasswd", util.EscapeShellArg(conf.RootPassword)))
	}

	return strings.Join(commands, "\n")
}

func writeSandboxFiles(rootfsPath, name string) error {
	hostnamePath := filepath.Join(rootfsPath, "etc", "hostname")
	if err := os.WriteFile(hostnamePath, []byte(name+"\n"), 0o644); err != nil {
		return fmt.Errorf("failed to write %v: %w", hostnamePath, err)
	}

	hostsPath := filepath.Join(rootfsPath, "etc", "hosts")
	hosts := fmt.Sprintf(`127.0.0.1	localhost
127.0.1.1	%v

# The following lines are desirable for IPv6 capable hosts
::1	localhost ip6-localhost ip6-loopback
ff02::1	ip6-allnodes
ff02::2	ip6-allrouters
`, name)
	if err := os.WriteFile(hostsPath, []byte(hosts), 0o644); err != nil {
		return fmt.Errorf("failed to write %v: %w", hostsPath, err)
	}

	return nil
}

func createDir(elem ...string) (string, error) {
	path := filepath.Join(elem...)
	if err := os.MkdirAll(path, 0o755); err != nil {
		return "", fmt.Errorf("failed to create directory %v: %w", path, err)
	}
	return path, nil
}

func init() {
	rootCmd.AddCommand(createCmd)
}
