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

var statusCmd = &cobra.Command{
	Use:   "status [name]",
	Short: "Show global status or status of a specific sandbox",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return showGlobalStatus()
		}
		return showSandboxStatus(args[0])
	},
}

func showGlobalStatus() error {
	fmt.Println("--- Host Commands ---")
	commands := []string{
		"systemctl", "systemd-nspawn", "systemd-run", "machinectl", "tar", "zstd", "ip",
	}

	for _, c := range commands {
		status := "OK"
		if _, err := exec.LookPath(c); err != nil {
			status = "MISSING"
		}
		fmt.Printf("%-18s %s\n", c+":", status)
	}

	mmStatus := "MISSING"
	if _, err := exec.LookPath("mmdebstrap"); err == nil {
		mmStatus = "OK"
	}
	fmt.Printf("%-18s %s (Primary)\n", "mmdebstrap:", mmStatus)

	debStatus := "MISSING"
	if _, err := exec.LookPath("debootstrap"); err == nil {
		debStatus = "OK"
	}
	fmt.Printf("%-18s %s (Fallback)\n", "debootstrap:", debStatus)

	fmt.Println("\n--- Networking ---")
	networkdStatus := "INACTIVE"
	if err := exec.Command("systemctl", "is-active", "--quiet", "systemd-networkd").Run(); err == nil {
		networkdStatus = "ACTIVE"
	}
	fmt.Printf("%-18s %s\n", "systemd-networkd:", networkdStatus)

	ipForwardStatus := "DISABLED"
	if data, err := os.ReadFile("/proc/sys/net/ipv4/ip_forward"); err == nil {
		if strings.TrimSpace(string(data)) == "1" {
			ipForwardStatus = "ENABLED"
		}
	}
	fmt.Printf("%-18s %s\n", "IP Forwarding:", ipForwardStatus)

	fmt.Println("\n--- Sandbox User ---")
	conf, err := config.Load(rootDir, "")
	if err != nil {
		fmt.Printf("Error loading global config: %v\n", err)
	} else {
		u, err := user.Lookup(conf.SandboxUser)
		if err != nil {
			fmt.Printf("SANDBOX_USER '%s': MISSING (does not exist on host)\n", conf.SandboxUser)
		} else {
			fmt.Printf("SANDBOX_USER '%s': OK (UID: %s)\n", conf.SandboxUser, u.Uid)
		}
	}

	fmt.Println("\n--- Existing Rootfs ---")
	entries, err := os.ReadDir(rootDir)
	if err != nil {
		fmt.Printf("Error reading root directory: %v\n", err)
	} else {
		found := false
		for _, entry := range entries {
			if entry.IsDir() && strings.HasPrefix(entry.Name(), "rootfs-") {
				fmt.Println(entry.Name())
				found = true
			}
		}
		if !found {
			fmt.Println("(none)")
		}
	}

	fmt.Println("\n--- Running Sandboxes ---")
	machineCmd := exec.Command("machinectl", "list") // todo: json is preferred for all machinectl output
	output, err := machineCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to query sandbox status with machinectl: %v", err)
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) > 0 {
		header := lines[0]
		fmt.Println(header)

		count := 0
		for i := 1; i < len(lines); i++ {
			line := strings.TrimSpace(lines[i])
			if strings.HasPrefix(line, "opqu-sbx-") {
				fmt.Println(lines[i])
				count++
			}
		}
		fmt.Printf("%d machines listed.\n", count)
	}

	return nil
}

func showSandboxStatus(name string) error {
	if err := sandbox.ValidateName(name); err != nil {
		return err
	}

	fmt.Printf("Sandbox: %s\n", name)

	rootfs := filepath.Join(rootDir, "rootfs-"+name)
	if _, err := os.Stat(rootfs); err == nil {
		fmt.Println("Rootfs Existence:   EXISTS")
	} else {
		fmt.Println("Rootfs Existence:   MISSING")
	}

	tarball := filepath.Join(rootDir, fmt.Sprintf("rootfs-%s.base.tar.zst", name))
	if _, err := os.Stat(tarball); err == nil {
		fmt.Println("Base Image:  EXISTS") // todo: printout the base image name and check if it's a file
	} else {
		fmt.Println("Base Image:  MISSING")
	}

	running, err := sandbox.IsRunning(name)
	if err != nil {
		fmt.Printf("Running:     ERROR (%v)\n", err)
	} else if running {
		fmt.Println("Running:     YES")
	} else {
		fmt.Println("Running:     NO")
	}

	conf, err := config.Load(rootDir, name) // todo: should check runtime mapping instead of static file
	if err == nil && conf.Ports != "" {
		fmt.Printf("Ports:       %s\n", conf.Ports)
	} else {
		fmt.Println("Ports:       (none)")
	}

	fmt.Println("Configuration:")
	configs := []string{name + ".conf", name + ".packages", name + ".mounts"}
	for _, c := range configs {
		path := filepath.Join(rootDir, "conf", c)
		status := "MISSING"
		if _, err := os.Stat(path); err == nil {
			status = "EXISTS"
		}
		fmt.Printf("  %-12s %s\n", c+":", status)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
