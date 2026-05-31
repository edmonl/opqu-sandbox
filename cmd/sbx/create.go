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
	Short: "Create rootfs for a new sandbox and its base snapshot",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if err := sandbox.ValidateName(name); err != nil {
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

		rootfsDir, err := sandbox.MkdirAllAsUser(conf, sbxDir, "rootfs")
		if err != nil {
			return err
		}
		snapshotsDir, err := sandbox.MkdirAllAsUser(conf, sbxDir, "snapshots", name)
		if err != nil {
			return err
		}
		pkgCache, err := sandbox.MkdirAllAsUser(conf, sbxDir, "pkg-cache")
		if err != nil {
			return err
		}

		// sudo
		if e := sandbox.Sudo(sbxDir); e != nil {
			return e
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
				util.Warn("failed to clean up %v: %v", sandboxFs, err)
				return
			}
			if hasMounts {
				util.Warn("failed to clean up %v: active mounts detected", sandboxFs)
				return
			}
			if err := os.RemoveAll(sandboxFs); err != nil {
				util.Warn("failed to clean up %v: %v", sandboxFs, err)
			}
		}()

		if _, err := exec.LookPath("mmdebstrap"); err == nil {
			mmdebstrapArgs := []string{
				"--variant=" + conf.Variant,
				"--skip=essential/unlink",
				"--setup-hook=mkdir -p \"$1/var/cache/apt/archives/\"",
				"--setup-hook=" + fmt.Sprintf("sync-in %q /var/cache/apt/archives/", pkgCache),
				"--customize-hook=" + fmt.Sprintf("sync-out /var/cache/apt/archives %q", pkgCache),
			}

			if len(packages) > 0 {
				mmdebstrapArgs = append(mmdebstrapArgs, "--include="+strings.Join(packages, ","))
			}

			mmdebstrapArgs = append(mmdebstrapArgs, conf.Distro, sandboxFs, conf.Mirror)
			if err := util.RunCmd("mmdebstrap", mmdebstrapArgs...); err != nil {
				return fmt.Errorf("provisioning sandbox %v with mmdebstrap failed: %w", name, err)
			}
			// Clean up apt partial directory to prevent permission errors for unprivileged users
			if err := os.RemoveAll(filepath.Join(pkgCache, "partial")); err != nil {
				util.Warn("failed to delete pkg-cache/partial: %v", err)
			}

			if entries, err := os.ReadDir(pkgCache); err != nil {
				util.Warn("failed to restore pkg-cache ownership: %v", err)
			} else {
				for _, entry := range entries {
					if !entry.Type().IsRegular() {
						util.Warn("unexpected entry %v in pkg-cache", entry.Name())
						continue
					}
					if err := util.ChownToUser(filepath.Join(pkgCache, entry.Name()), conf.SandboxUser); err != nil {
						util.Warn("%v", err)
					}
				}
			}
		} else if _, err := exec.LookPath("debootstrap"); err == nil {
			debootstrapArgs := []string{
				"--variant=" + conf.Variant,
			}

			if len(packages) > 0 {
				debootstrapArgs = append(debootstrapArgs, "--include="+strings.Join(packages, ","))
			}

			debootstrapArgs = append(debootstrapArgs, conf.Distro, sandboxFs, conf.Mirror)
			if err := util.RunCmd("debootstrap", debootstrapArgs...); err != nil {
				return fmt.Errorf("provisioning sandbox %v with debootstrap failed: %w", name, err)
			}

		} else {
			return fmt.Errorf("neither mmdebstrap nor debootstrap found in PATH")
		}

		// Provision sandbox user.
		if err := util.RunCmd("chroot", sandboxFs, "/bin/sh", "-c", getSetupScript(conf)); err != nil {
			return fmt.Errorf("failed to provision sandbox: %w", err)
		}

		// Enable networking
		if err := util.RunCmd("chroot", sandboxFs, "systemctl", "enable", "systemd-networkd"); err != nil {
			return fmt.Errorf("failed to enable systemd-networkd: %w", err)
		}

		if err := writeSandboxFiles(sandboxFs, name); err != nil {
			return err
		}

		if err := sandbox.CreateSymlink(sandboxFs, filepath.Join(conf.ImagesPath, name)); err != nil {
			return fmt.Errorf("failed to create sandbox image symlink: %w", err)
		}
		createSuccess = true

		if err := sandbox.CreateSnapshot(sandboxFs, snapshotsDir, "base", conf.SandboxUser); err != nil {
			util.Warn("failed to create base snapshot: %v", err)
		}

		return nil
	},
}

func getSetupScript(conf *config.Config) string {
	commands := []string{"set -e"}

	uid := conf.SandboxUser.UID
	if uid != 0 {
		gid := conf.SandboxUser.GID
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

func init() {
	rootCmd.AddCommand(createCmd)
}
