package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
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

func getCommandStatus(cmd string) string {
	status := "OK"
	if _, err := exec.LookPath(cmd); err != nil {
		status = "MISSING"
	}
	return status
}

func showGlobalStatus() error {
	var hasError bool

	fmt.Println("Running Sandboxes:")
	machineCmd := exec.Command("machinectl", "list", "--output=json")
	if output, err := machineCmd.CombinedOutput(); err == nil {
		var machines []map[string]any
		if jsonErr := json.Unmarshal(output, &machines); jsonErr != nil {
			fmt.Printf("failed to parsing machinectl JSON: %v\n", jsonErr)
			hasError = true
		} else {
			// todo: review this
			fmt.Printf("%-20v %-10v %-15v %-10v\n", "MACHINE", "CLASS", "SERVICE", "OS")
			count := 0
			for _, m := range machines {
				machine, _ := m["machine"].(string)
				if strings.HasPrefix(machine, "opqu-sbx-") {
					class, _ := m["class"].(string)
					service, _ := m["service"].(string)
					osName, _ := m["os"].(string)
					fmt.Printf("%-20v %-10v %-15v %-10v\n", machine, class, service, osName)
					count++
				}
			}
			fmt.Printf("\n%v machines listed.\n", count)
		}
	} else {
		if len(output) > 0 {
			fmt.Println(string(output))
		}
		fmt.Printf("failed to run machinectl: %v\n", err)
		hasError = true
	}

	fmt.Println("\nExisting Rootfs:")
	rootfsDir := filepath.Join(rootDir, "rootfs")
	if entries, err := os.ReadDir(rootfsDir); err == nil {
		found := false
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() {
				fmt.Println(name)
				found = true
			}
		}
		if !found {
			fmt.Println("(none)")
		}
	} else if errors.Is(err, fs.ErrNotExist) {
		fmt.Println("(none)")
	} else {
		fmt.Printf("failed to read sandbox rootfs directory: %v\n", err)
		hasError = true
	}

	fmt.Print("\nSandbox User")
	if conf, err := config.LoadConf(rootDir, ""); err == nil {
		u, userErr := user.Lookup(conf.SandboxUser);
		if  userErr == nil {
			fmt.Printf("%v (UID: %v): OK\n", u.Name, u.Uid)
		} else {
			if _, res := errors.AsType[user.UnknownUserError](userErr); res {
				fmt.Printf("%v: MISSING\n", conf.SandboxUser)
			} else {
				fmt.Printf(": failed to look up user %v on host: %v\n", conf.SandboxUser, userErr)
			}
		}
	} else {
		fmt.Printf(": failed to load default configuration: %v\n", err)
		hasError = true
	}

	fmt.Println("\n--- Host Commands ---")
	commands := []string{
		"systemctl", "systemd-nspawn", "systemd-run", "machinectl", "tar", "zstd", "ip",
	}
	for _, c := range commands {
		status := getCommandStatus(c)
		fmt.Printf("%-18v %v\n", c+":", status)
	}
	fmt.Printf("%-18v %v (Primary)\n", "mmdebstrap:", getCommandStatus("mmdebstrap"))
	fmt.Printf("%-18v %v (Fallback)\n", "debootstrap:", getCommandStatus("debootstrap"))

	fmt.Println("\n--- Networking ---")
	networkdStatus := "INACTIVE"
	if err := exec.Command("systemctl", "is-active", "--quiet", "systemd-networkd").Run(); err == nil {
		networkdStatus = "ACTIVE"
	} else if _, ok := errors.AsType[*exec.ExitError](err); !ok {
		networkdStatus = fmt.Sprintf("failed to run systemctl: %v", err)
	}
	fmt.Printf("%-18v %v\n", "systemd-networkd:", networkdStatus)

	ipForwardStatus := "DISABLED"
	if data, err := os.ReadFile("/proc/sys/net/ipv4/ip_forward"); err == nil {
		if strings.TrimSpace(string(data)) == "1" {
			ipForwardStatus = "ENABLED"
		}
	} else {
		ipForwardStatus = fmt.Sprintf("failed to read /proc/sys/net/ipv4/ip_forward: %v", err)
		hasError = true
	}
	fmt.Printf("%-18v %v\n", "IP Forwarding:", ipForwardStatus)


	if hasError {
		return errors.New("status gathering encountered issues")
	}
	return nil
}


func showSandboxStatus(name string) error {
	if err := sandbox.ValidateName(name); err != nil {
		return err
	}

	var errs []string
	fmt.Printf("Sandbox: %v\n", name)

	rootfs := filepath.Join(rootDir, "rootfs", name)
	if _, err := os.Stat(rootfs); err == nil {
		fmt.Println("Rootfs Existence:   EXISTS")
	} else {
		fmt.Println("Rootfs Existence:   MISSING")
		errs = append(errs, "rootfs missing")
	}

	tarball := filepath.Join(rootDir, "rootfs", fmt.Sprintf("%s.base.tar.zst", name))
	if info, err := os.Stat(tarball); err == nil {
		if info.Mode().IsRegular() {
			fmt.Printf("Base Image:  EXISTS (%v)\n", filepath.Base(tarball))
		} else {
			fmt.Printf("Base Image:  EXISTS but is not a regular file (%v)\n", filepath.Base(tarball))
			errs = append(errs, "base image is not a regular file")
		}
	} else {
		fmt.Println("Base Image:  MISSING")
		errs = append(errs, "base image missing")
	}

	running, err := sandbox.IsRunning(name)
	if err != nil {
		fmt.Printf("Running:     ERROR (%v)\n", err)
		errs = append(errs, fmt.Sprintf("failed to check if running: %v", err))
	} else if running {
		fmt.Println("Running:     YES")
	} else {
		fmt.Println("Running:     NO")
	}

	conf, err := config.LoadConf(rootDir, name)
	if err == nil {
		var displayPorts string
		portSource := "(config)"

		if running {
			if rtPorts := getRuntimePorts(name); rtPorts != "" {
				displayPorts = rtPorts
				portSource = "(runtime)"
			}
		}

		if displayPorts == "" && len(conf.Ports) > 0 {
			displayPorts = strings.Join(conf.Ports, " ")
		}

		if displayPorts != "" {
			fmt.Printf("Ports %-7v %v\n", portSource+":", displayPorts)
		} else {
			fmt.Println("Ports:       (none)")
		}
	} else {
		fmt.Println("Ports:       (unknown - config error)")
		errs = append(errs, fmt.Sprintf("failed to load sandbox config: %v", err))
	}

	fmt.Println("Configuration:")
	configs := []string{name + ".conf", name + ".packages", name + ".mounts"}
	for _, c := range configs {
		path := filepath.Join(rootDir, "conf", c)
		status := "MISSING"
		if _, err := os.Stat(path); err == nil {
			status = "EXISTS"
		}
		fmt.Printf("  %-12v %v\n", c+":", status)
	}

	if len(errs) > 0 {
		return fmt.Errorf("sandbox status encountered issues:\n- %v", strings.Join(errs, "\n- "))
	}
	return nil
}

func getRuntimePorts(name string) string {
	machine := sandbox.MachineName(name)
	cmd := exec.Command("systemctl", "show", machine+".service", "-p", "ExecStart", "--value")
	output, err := cmd.CombinedOutput()
	if err != nil || len(output) == 0 {
		return ""
	}

	outStr := string(output)
	var ports []string
	fields := strings.Fields(outStr)
	for _, f := range fields {
		if strings.HasPrefix(f, "--port=") {
			p := strings.TrimPrefix(f, "--port=")
			p = strings.TrimRight(p, ";")
			ports = append(ports, p)
		}
	}
	return strings.Join(ports, " ")
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
