package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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

type machineInfo struct {
	Machine   string `json:"machine"`
	Class     string `json:"class"`
	Service   string `json:"service"`
	OS        string `json:"os"`
	Version   string `json:"version"`
	Addresses string `json:"addresses"`
}

func getCommandStatus(cmd string) string {
	status := "ok"
	if _, err := exec.LookPath(cmd); err != nil {
		status = "not available"
	}
	return status
}

const (
	normalFormat = "%-18v %v\n"
	errFormat    = "%-18v ERROR (%v)\n"
)

func showGlobalStatus() error {
	var format string
	var status any
	var hasError bool

	fmt.Println("Running Sandboxes:")
	machineCmd := exec.Command("machinectl", "list", "--output=json")
	if output, err := machineCmd.CombinedOutput(); err == nil {
		var machines []machineInfo
		if jsonErr := json.Unmarshal(output, &machines); jsonErr != nil {
			fmt.Printf("failed to parsing machinectl JSON: %v\n", jsonErr)
			hasError = true
		} else {
			type row struct {
				sandbox   string
				os        string
				addresses string
			}
			var rows []row
			maxSandbox, maxOS := 7, 2 // lengths of "SANDBOX" and "OS"
			prefix := sandbox.MachineName("")

			for _, m := range machines {
				if strings.HasPrefix(m.Machine, prefix) && m.Class == "container" && m.Service == "systemd-nspawn" {
					name := strings.TrimPrefix(m.Machine, prefix)
					osInfo := m.OS
					if m.Version != "" {
						if osInfo != "" {
							osInfo += " "
						}
						osInfo += m.Version
					}
					if osInfo == "" {
						osInfo = "-"
					}
					addr := strings.ReplaceAll(strings.TrimSpace(m.Addresses), "\n", ", ")
					if addr == "" {
						addr = "-"
					}
					rows = append(rows, row{name, osInfo, addr})

					maxSandbox = max(len(name), maxSandbox)
					maxOS = max(len(osInfo), maxOS)
				}
			}

			if len(rows) == 0 {
				fmt.Println("(none)")
			} else {
				rowFormat := fmt.Sprintf("%%-%dv %%-%dv %%v\n", maxSandbox+1, maxOS+1)
				fmt.Printf(rowFormat, "SANDBOX", "OS", "ADDRESSES")
				for _, r := range rows {
					fmt.Printf(rowFormat, r.sandbox, r.os, r.addresses)
				}
				fmt.Printf("%v sandbox(es) listed.\n", len(rows))
			}
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

	fmt.Println()
	if conf, err := config.LoadConf(rootDir, ""); err == nil {
		u := conf.SandboxUser
		format = normalFormat
		status = fmt.Sprintf("%v (UID %v) ok", u.Username, u.Uid)
	} else {
		format = errFormat
		status = err
		hasError = true
	}
	fmt.Printf(format, "Sandbox User:", status)

	fmt.Println("\nHost Commands:")
	commands := []string{"mmdebstrap", "debootstrap", "systemctl", "systemd-nspawn", "systemd-run", "machinectl", "sudo", "su"}
	for _, c := range commands {
		status := getCommandStatus(c)
		fmt.Printf(normalFormat, c+":", status)
	}

	fmt.Println("\nNetworking:")
	out, _ := exec.Command("systemctl", "is-active", "systemd-networkd").CombinedOutput()
	networkdStatus := strings.TrimSpace(string(out))
	if networkdStatus == "" {
		networkdStatus = "UNKNOWN"
		hasError = true
	}
	fmt.Printf(normalFormat, "systemd-networkd:", networkdStatus)

	ipForwardStatus := "disabled"
	if data, err := os.ReadFile("/proc/sys/net/ipv4/ip_forward"); err == nil {
		if strings.TrimSpace(string(data)) == "1" {
			ipForwardStatus = "enabled"
		}
	} else {
		ipForwardStatus = fmt.Sprintf("failed to read /proc/sys/net/ipv4/ip_forward: %v", err)
		hasError = true
	}
	fmt.Printf(normalFormat, "IP Forwarding:", ipForwardStatus)

	if hasError {
		return errors.New("status gathering encountered issues")
	}
	return nil
}

func showSandboxStatus(name string) error {
	if err := sandbox.ValidateName(name); err != nil {
		return err
	}

	fmt.Printf(normalFormat, "Machine Name:", sandbox.MachineName(name))

	rootfs := filepath.Join(rootDir, "rootfs")

	var format string
	var status any
	var hasError bool

	if _, err := os.Stat(filepath.Join(rootfs, name)); err == nil {
		format = normalFormat
		status = "ok"
	} else if errors.Is(err, fs.ErrNotExist) {
		format = normalFormat
		status = "missing"
	} else {
		format = errFormat
		status = err
		hasError = true
	}
	fmt.Printf(format, "Rootfs:", status)

	if info, err := os.Stat(filepath.Join(rootfs, fmt.Sprintf("%v.base.tar.zst", name))); err == nil {
		if info.Mode().IsRegular() {
			format = normalFormat
			status = "ok"
		} else {
			format = errFormat
			status = "not a regular file"
		}
	} else if errors.Is(err, fs.ErrNotExist) {
		format = normalFormat
		status = "missing"
	} else {
		format = errFormat
		status = err
		hasError = true
	}
	fmt.Printf(format, "Base Image:", status)

	if running, err := sandbox.IsRunning(name); err == nil {
		format = normalFormat
		if running {
			status = "yes"
		} else {
			status = "no"
		}
	} else {
		format = errFormat
		status = err
		hasError = true
	}
	fmt.Printf(format, "Running:", status)

	fmt.Println("\nConfiguration Files:")
	confDir := filepath.Join(rootDir, "conf")
	configs := []string{name + ".conf", name + ".packages", name + ".mounts"}
	for _, c := range configs {
		path := filepath.Join(confDir, c)
		if _, err := os.Stat(path); err == nil {
			fmt.Println(c)
		} else if !errors.Is(err, fs.ErrNotExist) {
			fmt.Printf("%v: %v\n", c, err)
			hasError = true
		}
	}

	fmt.Println("\nConfiguration:")
	conf, confErr := config.LoadConf(rootDir, name)
	if confErr == nil {
		fmt.Printf(normalFormat, "DISTRO:", conf.Distro)
		fmt.Printf(normalFormat, "MIRROR:", conf.Mirror)
		fmt.Printf(normalFormat, "VARIANT:", conf.Variant)
		fmt.Printf(normalFormat, "NETWORK_ZONE:", conf.NetworkZone)
		fmt.Printf(normalFormat, "RESOLV_CONF:", conf.ResolvConf)
		fmt.Printf(normalFormat, "SANDBOX_USER:", fmt.Sprintf("%v (UID %v)", conf.SandboxUser.Username, conf.SandboxUser.Uid))
		rootPasswordStatus := "(locked)"
		if conf.RootPassword != "" {
			rootPasswordStatus = "(configured)"
		}
		fmt.Printf(normalFormat, "ROOT_USER_PASSWORD:", rootPasswordStatus)
		ports := "(none)"
		if len(conf.Ports) > 0 {
			ports = strings.Join(conf.Ports, " ")
		}
		fmt.Printf(normalFormat, "PORTS:", ports)
	} else {
		fmt.Printf("failed to load configuration: %v\n", confErr)
		hasError = true
	}

	fmt.Println("\nPackages:")
	if packages, err := config.LoadPackages(rootDir, name); err == nil {
		if len(packages) == 0 {
			fmt.Println("(none)")
		} else {
			slices.Sort(packages)
			for _, p := range packages {
				fmt.Println(p)
			}
		}
	} else {
		fmt.Printf("failed to load packages: %v\n", err)
		hasError = true
	}

	fmt.Println("\nMounts:")
	if conf == nil || conf.SandboxUser == nil {
		fmt.Println("failed to load mounts: no successfully loaded configuration")
		hasError = true
	} else if mounts, err := config.LoadMounts(rootDir, name, conf.SandboxUser); err == nil {
		if len(mounts) == 0 {
			fmt.Println("(none)")
		} else {
			for _, m := range mounts {
				ro := ""
				if m.ReadOnly {
					ro = ":ro"
				}
				fmt.Printf("%v:%v%v\n", m.HostPath, m.SandboxPath, ro)
			}
		}
	} else {
		fmt.Printf("failed to load mounts: %v\n", err)
		hasError = true
	}

	if hasError {
		return errors.New("status gathering encountered issues")
	}
	return nil
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
