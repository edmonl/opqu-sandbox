package config

import (
	"bufio"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/joho/godotenv"
)

type Mount struct {
	HostPath      string
	SandboxPath string
	ReadOnly      bool
}

type Config struct {
	Distro        string
	Mirror        string
	Variant       string
	SandboxUser   string
	Ports         string
	Audio         bool
	ResolvConf    string
	Packages      []string
	Mounts        []Mount
}

func Load(rootDir, name string) (*Config, error) {
	conf := &Config{
		Distro:     "trixie",
		Mirror:     "http://deb.debian.org/debian",
		Variant:    "standard",
		Audio:      false,
		ResolvConf: "auto",
	}

	u, err := user.Current()
	if err == nil {
		conf.SandboxUser = u.Username
	}

	// 1. Load global.conf
	globalPath := filepath.Join(rootDir, "conf", "global.conf")
	if _, err := os.Stat(globalPath); err == nil {
		globalEnv, _ := godotenv.Read(globalPath)
		applyEnv(conf, globalEnv)
	}

	// 2. Load <name>.conf, <name>.packages, <name>.mounts
	if name != "" {
		sandboxPath := filepath.Join(rootDir, "conf", name+".conf")
		if _, err := os.Stat(sandboxPath); err == nil {
			sandboxEnv, _ := godotenv.Read(sandboxPath)
			applyEnv(conf, sandboxEnv)
		}

		packagesPath := filepath.Join(rootDir, "conf", name+".packages")
		conf.Packages = loadPackages(packagesPath)

		mountsPath := filepath.Join(rootDir, "conf", name+".mounts")
		conf.Mounts = loadMounts(mountsPath)
	}

	return conf, nil
}

func applyEnv(c *Config, env map[string]string) {
	if v, ok := env["DISTRO"]; ok {
		c.Distro = v
	}
	if v, ok := env["MIRROR"]; ok {
		c.Mirror = v
	}
	if v, ok := env["VARIANT"]; ok {
		c.Variant = v
	}
	if v, ok := env["SANDBOX_USER"]; ok && v != "" {
		c.SandboxUser = v
	}
	if v, ok := env["PORTS"]; ok {
		c.Ports = v
	}
	if v, ok := env["AUDIO"]; ok {
		c.Audio = (v == "yes")
	}
	if v, ok := env["RESOLV_CONF"]; ok && v != "" {
		c.ResolvConf = v
	}
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
				HostPath:      matches[1],
				SandboxPath: matches[2],
				ReadOnly:      matches[3] == ":ro",
			})
		}
	}
	return mounts
}
