// Package config provides functions to load configuration from the sandbox directory.
package config

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
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
	Distro       string
	Mirror       string
	Variant      string
	SandboxUser  *user.User
	Ports        []string
	NetworkZone  string
	ResolvConf   string
	RootPassword string
	ImagesPath   string
}

var zoneRegex = regexp.MustCompile(`^[a-z0-9-]+$`)
var passwordRegex = regexp.MustCompile(`^\P{C}+$`)

func loadConfFile(path string) (map[string]string, error) {
	conf, err := godotenv.Read(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("failed to load %v: %w", path, err)
	}

	return conf, nil
}

func LoadConf(sbxDir, name string) (*Config, error) {
	// Load default conf
	rawConf, err := loadConfFile(filepath.Join(sbxDir, "conf", "default"))
	if err != nil {
		return nil, err
	}

	// Load <name>.conf
	if name != "" {
		sandboxConf, err := loadConfFile(filepath.Join(sbxDir, "conf", name+".conf"))
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
		Distro:       "stable",
		Mirror:       "http://deb.debian.org/debian",
		Variant:      "standard",
		NetworkZone:  "opqu-sbx",
		ResolvConf:   "auto",
		ImagesPath:   "/var/lib/machines",
	}

	if v := rawConf["DISTRO"]; v != "" {
		conf.Distro = v
	}
	if v := rawConf["MIRROR"]; v != "" {
		conf.Mirror = v
	}
	if v := rawConf["VARIANT"]; v != "" {
		conf.Variant = v
	}
	if v := rawConf["NETWORK_ZONE"]; v != "" {
		if !zoneRegex.MatchString(v) {
			return nil, errors.New("failed to load configuration: NETWORK_ZONE must be lowercase alphanumeric and hyphens only")
		}
		if len(v) > 12 {
			return nil, errors.New("failed to load configuration: NETWORK_ZONE is limited to 12 characters")
		}
		conf.NetworkZone = v
	}
	if v := rawConf["RESOLV_CONF"]; v != "" {
		conf.ResolvConf = v
	}
	if v := rawConf["ROOT_USER_PASSWORD"]; v != "" {
		if !passwordRegex.MatchString(v) {
			return nil, errors.New("failed to load configuration: ROOT_USER_PASSWORD must contain only visible characters and no control characters")
		}
		conf.RootPassword = v
	}
	if v := rawConf["IMAGES_PATH"]; v != "" {
		if !filepath.IsAbs(v) {
			return nil, errors.New("failed to load configuration: IMAGES_PATH must be an absolute path")
		}
		conf.ImagesPath = v
	}

	if v := rawConf["PORTS"]; v != "" {
		ports, err := parsePorts(v)
		if err != nil {
			return nil, err
		}
		conf.Ports = ports
	}

	userName := rawConf["SANDBOX_USER"]
	if userName == "" && os.Geteuid() == 0 {
		userName = os.Getenv("SUDO_USER")
	}

	if userName == "" {
		if u, err := user.Current(); err == nil {
			conf.SandboxUser = u
		} else {
			return nil, fmt.Errorf("failed to get current user: %w", err)
		}
	} else if u, err := user.Lookup(userName); err == nil {
		conf.SandboxUser = u
	} else {
		return nil, fmt.Errorf("failed to find user %v: %w", userName, err)
	}

	return conf, nil
}

var portRegex = regexp.MustCompile(`^((tcp|udp):)?\d+(:\d+)?$`)

func parsePorts(v string) ([]string, error) {
	var ports []string
	for f := range strings.FieldsSeq(v) {
		if !portRegex.MatchString(f) {
			return nil, fmt.Errorf("failed to load configuration: invalid port mapping: %v", f)
		}
		ports = append(ports, f)
	}
	return ports, nil
}

func loadLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}

		return nil, fmt.Errorf("failed to load %v: %w", path, err)
	}
	defer f.Close()

	uniqLines := map[string]struct{}{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if idx := strings.Index(line, "#"); idx != -1 {
			line = line[:idx]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		uniqLines[line] = struct{}{}
	}

	err = scanner.Err()
	if err != nil {
		return nil, fmt.Errorf("failed to read %v: %w", path, err)
	}

	lines := make([]string, 0, len(uniqLines))
	for line := range uniqLines {
		lines = append(lines, line)
	}

	return lines, nil
}

var packageRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9+.-]+$`)

func LoadPackages(sbxDir, name string) ([]string, error) {
	packagesPath := filepath.Join(sbxDir, "conf", name+".packages")
	packages, err := loadLines(packagesPath)
	if err != nil {
		return nil, err
	}

	for _, p := range packages {
		if !packageRegex.MatchString(p) {
			return nil, fmt.Errorf("invalid package %v in %v", p, packagesPath)
		}
	}

	return packages, nil
}

var mountRegex = regexp.MustCompile(`^([^:]*)(?::([^:]*))?(:ro)?$`)

func LoadMounts(sbxDir, name string, u *user.User) ([]*Mount, error) {
	mountsPath := filepath.Join(sbxDir, "conf", name+".mounts")
	mountLines, err := loadLines(mountsPath)
	if err != nil {
		return nil, err
	}

	sandboxPathMounts := map[string]*Mount{}
	for _, m := range mountLines {
		matches := mountRegex.FindStringSubmatch(m)
		if matches == nil {
			return nil, fmt.Errorf("invalid mount %v in %v", m, mountsPath)
		}

		hostPath := strings.TrimSpace(matches[1])
		sandboxPath := strings.TrimSpace(matches[2])
		readOnly := matches[3] != ""

		if hostPath == "" && sandboxPath == "" {
			return nil, fmt.Errorf("invalid mount %v in %v", m, mountsPath)
		}

		if hostPath != "" {
			if strings.HasPrefix(hostPath, "~") {
				if len(hostPath) > 1 && hostPath[1] != os.PathSeparator {
					return nil, fmt.Errorf("invalid mount %v in %v", m, mountsPath)
				}

				if u.HomeDir == "" {
					return nil, fmt.Errorf("invalid mount %v in %v: user %v does not have a home directory", m, mountsPath, u.Username)
				}

				hostPath = filepath.Join(u.HomeDir, hostPath[min(2, len(hostPath)):])
			} else if !filepath.IsAbs(hostPath) {
				hostPath = filepath.Join(sbxDir, hostPath)
			}
		}

		if sandboxPath == "" {
			sandboxPath = hostPath
		} else if !filepath.IsAbs(sandboxPath) {
			return nil, fmt.Errorf("invalid mount %v in %v: sandbox path must be absolute", m, mountsPath)
		}

		mount := sandboxPathMounts[sandboxPath]
		if mount == nil {
			sandboxPathMounts[sandboxPath] = &Mount{
				HostPath:    hostPath,
				SandboxPath: sandboxPath,
				ReadOnly:    readOnly,
			}
		} else if mount.HostPath == hostPath {
			mount.ReadOnly = mount.ReadOnly || readOnly
		} else {
			return nil, fmt.Errorf("invalid mount %v in %v: same sandbox path is mounted to again", m, mountsPath)
		}
	}

	mounts := make([]*Mount, 0, len(sandboxPathMounts))
	for _, mount := range sandboxPathMounts {
		mounts = append(mounts, mount)
	}

	return mounts, nil
}
