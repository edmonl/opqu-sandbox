package sandbox

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

var nameRegex = regexp.MustCompile(`^[a-z0-9-]+$`)

func ValidateName(name string) error {
	if nameRegex.MatchString(name) {
		return nil
	}
	return fmt.Errorf("sandbox name %v is invalid, must be lowercase alphanumeric and hyphens only", name)
}

func MachineName(name string) string {
	return fmt.Sprintf("opqu-sbx-%v", name)
}

func BridgeName(zone string) string {
	return fmt.Sprintf("vz-%v", zone)
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

func RunCmd(cmd string, args ...string) error {
	execCmd := exec.Command(cmd, args...)
	execCmd.Stdin = os.Stdin
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr
	return execCmd.Run()
}
