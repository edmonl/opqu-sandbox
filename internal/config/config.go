// Package config provides functions to load configuration from the sandbox root directory.
package config

import (
	"bufio"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/joho/godotenv"
)

type Mount struct {
	HostPath    string
	SandboxPath string
	ReadOnly    bool
}

type Config struct {
	Distro      string
	Mirror      string
	Variant     string
	SandboxUser string
	Ports       []string
	Audio       bool
	ResolvConf  string
}

var (
	yesValues = map[string]struct{}{"yes": {}, "y": {}, "true": {}, "t": {}, "1": {}, "on": {}}
	noValues  = map[string]struct{}{"no": {}, "n": {}, "false": {}, "f": {}, "0": {}, "off": {}}
)

func loadConfFile(path string) (map[string]string, error) {
	conf, err := godotenv.Read(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load %v: %w", path, err)
	}

	return conf, nil
}

func LoadConf(rootDir, name string) (*Config, error) {
	// Load default conf
	rawConf, err := loadConfFile(filepath.Join(rootDir, "conf", "default"))
	if err != nil {
		return nil, err
	}

	// Load <name>.conf
	if name != "" {
		sandboxConf, err := loadConfFile(filepath.Join(rootDir, "conf", name+".conf"))
		if err != nil {
			return nil, err
		}
		for k, v := range sandboxConf {
			if v != "" {
				rawConf[k] = v
			}
		}
	}

	conf := &Config{
		Distro:     "trixie",
		Mirror:     "http://deb.debian.org/debian",
		Variant:    "standard",
		Audio:      false,
		ResolvConf: "auto",
	}

	if err := applyConf(conf, rawConf); err != nil {
		return nil, err
	}

	if u, err := user.Current(); err == nil {
		conf.SandboxUser = u.Username
	} else {
		return nil, fmt.Errorf("failed to get current user: %w", err)
	}

	return conf, nil
}

func LoadPackages(rootDir, name string) []string {
	if name == "" {
		return nil
	}
	packagesPath := filepath.Join(rootDir, "conf", name+".packages")
	return loadPackages(packagesPath)
}

func LoadMounts(rootDir, name string) []Mount {
	if name == "" {
		return nil
	}
	mountsPath := filepath.Join(rootDir, "conf", name+".mounts")
	return loadMounts(mountsPath)
}

func applyConf(c *Config, env map[string]string) error {
	if v := env["DISTRO"]; v != "" {
		c.Distro = v
	}
	if v := env["MIRROR"]; v != "" {
		c.Mirror = v
	}
	if v := env["VARIANT"]; v != "" {
		c.Variant = v
	}
	if v := env["SANDBOX_USER"]; v != "" {
		c.SandboxUser = v
	}
	if v := env["PORTS"]; v != "" {
		ports, err := parsePorts(v)
		if err != nil {
			return err
		}
		c.Ports = ports
	}
	if v := strings.ToLower(env["AUDIO"]); v != "" {
		_, ok := yesValues[v]
		c.Audio = ok
	}
	if v := env["RESOLV_CONF"]; v != "" {
		c.ResolvConf = v
	}
	return nil
}

func parsePorts(v string) ([]string, error) {
	var ports []string
	for f := range strings.FieldsSeq(v) {
		if !isValidPort(f) {
			return nil, fmt.Errorf("invalid port mapping: %s", f)
		}
		ports = append(ports, f)
	}
	return ports, nil
}

var portRegex = regexp.MustCompile(`^((tcp|udp):)?\d+(:\d+)?$`)

func isValidPort(p string) bool {
	return portRegex.MatchString(p)
}

func loadPackages(path string) []string {
	var packages []string
	f, err := os.Open(path)
	if err != nil {
		return packages
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		packages = append(packages, line)
	}
	return packages
}

func loadMounts(path string) []Mount {
	var mounts []Mount
	f, err := os.Open(path)
	if err != nil {
		return mounts
	}
	defer f.Close()

	mountRegex := regexp.MustCompile(`^([^:]+):([^:]+)(:ro)?$`)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		matches := mountRegex.FindStringSubmatch(line)
		if len(matches) > 0 {
			mounts = append(mounts, Mount{
				HostPath:    matches[1],
				SandboxPath: matches[2],
				ReadOnly:    matches[3] == ":ro",
			})
		}
	}
	return mounts
}
