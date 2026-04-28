package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

func sudo() error {
	// Only escalate if not already root
	if os.Geteuid() != 0 {
		fmt.Print("This operation requires root privileges. Press [Enter] directly to escalate, or Ctrl+C to cancel: ")
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err == io.EOF || strings.TrimSpace(input) != "" {
			if err == io.EOF {
				fmt.Println()
			}
			fmt.Println("Escalation cancelled.")
			os.Exit(0)
		}
		if err != nil {
			return err
		}

		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to resolve the executable path: %w", err)
		}

		// Re-run with sudo
		sudoCmd := exec.Command("sudo", append([]string{exe}, os.Args[1:]...)...)
		sudoCmd.Stdin = os.Stdin
		sudoCmd.Stdout = os.Stdout
		sudoCmd.Stderr = os.Stderr
		if err := sudoCmd.Run(); err != nil {
			return err
		}
		os.Exit(0)
	}
	return nil
}
