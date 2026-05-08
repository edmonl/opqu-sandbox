package sandbox

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/edmonl/opqu-sandbox/internal/util"
)

func Sudo(sbxDir string) error {
	// Only escalate if not already root
	if os.Geteuid() == 0 {
		return nil
	}

	var escalationCmd string
	if _, err := exec.LookPath("sudo"); err == nil {
		escalationCmd = "sudo"
	} else if _, err := exec.LookPath("su"); err == nil {
		escalationCmd = "su"
	} else {
		return errors.New("neither sudo nor su found in PATH")
	}

	prompt := fmt.Sprintf("This operation requires to invoke %v. Press [Enter] directly to continue, or Ctrl+C to cancel: ", escalationCmd)
	input, err := util.Confirm(prompt)
	if err != nil {
		return err
	}
	if input != "" {
		return fmt.Errorf("user cancelled invoking %v", escalationCmd)
	}

	exe, err := filepath.Abs(os.Args[0])
	if err != nil {
		return fmt.Errorf("failed to resolve the executable path: %w", err)
	}

	// Prepare arguments for escalation, ensuring the root directory is preserved.
	// We append the absolute rootDir to the end of the arguments. Since the
	// flag library follows the "last one wins" rule, this will correctly
	// override any previous --sbx-dir flags or environment variable defaults
	// in the escalated process.
	escalatedArgs := append(os.Args[1:], "--sbx-dir", sbxDir)

	var cmd *exec.Cmd
	if escalationCmd == "sudo" {
		cmd = exec.Command("sudo", append([]string{exe}, escalatedArgs...)...)
	} else {
		// su -c "exe args..."
		// We need to escape arguments for the shell
		var args []string
		args = append(args, util.EscapeShellArg(exe))
		for _, arg := range escalatedArgs {
			args = append(args, util.EscapeShellArg(arg))
		}

		// When using su, we should manually set SUDO_USER so LoadConf works
		u, userErr := user.Current()
		if userErr != nil {
			return fmt.Errorf("failed to get current user: %w", userErr)
		}
		cmd = exec.Command("su", "-c", strings.Join(args, " "))
		if u != nil {
			cmd.Env = append(os.Environ(), "SUDO_USER="+u.Username)
		}
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err == nil {
		os.Exit(0)
	}
	return err
}
