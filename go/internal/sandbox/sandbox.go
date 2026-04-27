package sandbox

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/edmonl/opqu-sandbox/internal/config"
)

var nameRegex = regexp.MustCompile(`^[a-z0-9-]{1,12}$`)

func ValidateName(name string) error {
	if !nameRegex.MatchString(name) {
		return fmt.Errorf("sandbox name '%s' is invalid (%d characters); must be 1–12 characters, lowercase alphanumeric and hyphens only", name, len(name))
	}
	return nil
}

func MachineName(name string) string {
	return fmt.Sprintf("opqu-sbx-%s", name)
}

func ZoneName(name string) string {
	basePrefix := "opqu"
	full := fmt.Sprintf("%s-%s", basePrefix, name)

	if len(full) <= 12 {
		return full
	}

	available := 12 - len(name)
	if available >= 2 {
		return fmt.Sprintf("%s-%s", basePrefix[:available-1], name)
	} else if available == 1 {
		return fmt.Sprintf("o%s", name)
	} else {
		return name[:12]
	}
}

func BridgeName(name string) string {
	return fmt.Sprintf("vz-%s", ZoneName(name))
}

func IsRunning(name string) (bool, error) {
	machine := MachineName(name)
	cmd := exec.Command("machinectl", "list", "--no-legend")
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to query sandbox state with machinectl: %v", err)
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == machine {
			return true, nil
		}
	}

	return false, nil
}

func BuildIncludeArg(conf *config.Config) []string {
	var packages []string
	seen := make(map[string]bool)

	for _, pkg := range conf.Packages {
		if !seen[pkg] {
			packages = append(packages, pkg)
			seen[pkg] = true
		}
	}

	if conf.Audio && !seen["pipewire-pulse"] {
		packages = append(packages, "pipewire-pulse")
	}

	return packages
}
