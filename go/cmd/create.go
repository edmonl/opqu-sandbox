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

var createCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Bootstrap a new sandbox and save its clean base image",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if err := sandbox.ValidateName(name); err != nil {
			return err
		}

		if strings.ContainsAny(rootDir, " \t\n\r") {
			return fmt.Errorf("sandbox root '%s' contains whitespace; move it to a path without spaces", rootDir)
		}

		rootfs := filepath.Join(rootDir, "rootfs-"+name)
		tarball := filepath.Join(rootDir, fmt.Sprintf("rootfs-%s.base.tar.zst", name))

		if _, err := os.Stat(rootfs); err == nil {
			return fmt.Errorf("sandbox '%s' already exists", name)
		}

		conf, err := config.Load(rootDir, name)
		if err != nil {
			return err
		}

		u, err := user.Lookup(conf.SandboxUser)
		if err != nil {
			return fmt.Errorf("user %s does not exist on the host.", conf.SandboxUser)
		}
		uid := u.Uid

		pkgCache := filepath.Join(rootDir, "pkg-cache")
		if err := os.MkdirAll(pkgCache, 0755); err != nil {
			return fmt.Errorf("failed to create pkg-cache directory: %v", err)
		}

		packages := sandbox.BuildIncludeArg(conf)

		if _, err := exec.LookPath("mmdebstrap"); err == nil {
			mmdebstrapArgs := []string{
				"--variant=" + conf.Variant,
				"--skip=essential/unlink",
				"--setup-hook=mkdir -p \"$1/var/cache/apt/archives/\"",
			}

			cacheInHook := fmt.Sprintf("sync-in %q /var/cache/apt/archives/", pkgCache)
			cacheOutHook := fmt.Sprintf("sync-out /var/cache/apt/archives %q", pkgCache)

			mmdebstrapArgs = append(mmdebstrapArgs,
				"--setup-hook="+cacheInHook,
				"--customize-hook="+cacheOutHook,
			)

			enableNetworkdHook := `chroot "$1" systemctl enable systemd-networkd`
			createUserCmd := fmt.Sprintf("useradd -m -u %s -s /bin/bash %s && passwd -l root && passwd -l %s", uid, conf.SandboxUser, conf.SandboxUser)
			createUserHook := fmt.Sprintf(`chroot "$1" /bin/sh -c %q`, createUserCmd)

			mmdebstrapArgs = append(mmdebstrapArgs,
				"--customize-hook="+enableNetworkdHook,
				"--customize-hook="+createUserHook,
			)

			if len(packages) > 0 {
				mmdebstrapArgs = append(mmdebstrapArgs, "--include="+strings.Join(packages, ","))
			}

			mmdebstrapArgs = append(mmdebstrapArgs, conf.Distro, rootfs, conf.Mirror)

			mmCmd := exec.Command("mmdebstrap", mmdebstrapArgs...)
			mmCmd.Stdout = os.Stdout
			mmCmd.Stderr = os.Stderr

			if err := mmCmd.Run(); err != nil {
				return fmt.Errorf("bootstrapping sandbox '%s' with mmdebstrap failed: %v", name, err)
			}
		} else if _, err := exec.LookPath("debootstrap"); err == nil {
			fmt.Println("mmdebstrap not found, falling back to debootstrap...")

			debootstrapArgs := []string{
				"--variant=" + conf.Variant,
			}

			if len(packages) > 0 {
				debootstrapArgs = append(debootstrapArgs, "--include="+strings.Join(packages, ","))
			}

			debootstrapArgs = append(debootstrapArgs, conf.Distro, rootfs, conf.Mirror)

			debCmd := exec.Command("debootstrap", debootstrapArgs...)
			debCmd.Stdout = os.Stdout
			debCmd.Stderr = os.Stderr

			if err := debCmd.Run(); err != nil {
				return fmt.Errorf("bootstrapping sandbox '%s' with debootstrap failed: %v", name, err)
			}

			// Enable networking
			networkCmd := exec.Command("chroot", rootfs, "systemctl", "enable", "systemd-networkd")
			if err := networkCmd.Run(); err != nil {
				return fmt.Errorf("failed to enable systemd-networkd: %v", err)
			}

			// Create user and lock root
			userScript := fmt.Sprintf("useradd -m -u %s -s /bin/bash %s && passwd -l root && passwd -l %s", uid, conf.SandboxUser, conf.SandboxUser)
			userCmd := exec.Command("chroot", rootfs, "/bin/sh", "-c", userScript)
			if err := userCmd.Run(); err != nil {
				return fmt.Errorf("failed to create sandbox user: %v", err)
			}
		} else {
			return fmt.Errorf("neither mmdebstrap nor debootstrap found in PATH")
		}

		tarCmd := exec.Command("tar", "--zstd", "-cf", tarball, "-C", rootDir, "rootfs-"+name+"/")
		tarCmd.Stdout = os.Stdout
		tarCmd.Stderr = os.Stderr

		if err := tarCmd.Run(); err != nil {
			return fmt.Errorf("failed to create base tarball for sandbox '%s': %v", name, err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(createCmd)
}
