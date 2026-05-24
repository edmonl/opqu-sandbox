// Package util provides general utility functions used across the application.
package util

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// Confirm prompts the user via stderr and reads a single line of response from stdin.
// It returns true if the user presses Enter directly (empty input).
// It returns false if the user enters any text or sends an EOF (e.g., Ctrl+D).
// Raw I/O errors are returned as the error.
func Confirm(prompt string) (bool, error) {
	fmt.Fprint(os.Stderr, prompt)
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		if err == io.EOF {
			fmt.Println()
			return false, nil
		}
		return false, err
	}

	return strings.TrimRight(input, "\r\n") == "", nil
}

// Warn writes a formatted warning message to stderr.
func Warn(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "Warning: "+format+"\n", args...)
}

// RunCmd executes a command with the provided arguments, binding its standard streams to the parent process.
// Raw errors are returned.
func RunCmd(cmd string, args ...string) error {
	execCmd := exec.Command(cmd, args...)
	execCmd.Stdin = os.Stdin
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr
	return execCmd.Run()
}

// CheckSymlinkTarget reports whether path is a symlink pointing exactly to wantTarget.
func CheckSymlinkTarget(path, wantTarget string) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return false, fmt.Errorf("failed to access %v: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return false, fmt.Errorf("%v is not a symlink", path)
	}

	target, err := os.Readlink(path)
	if err != nil {
		return false, fmt.Errorf("failed to read symlink %v: %w", path, err)
	}

	return target == wantTarget, nil
}

// EscapeShellArg wraps a string in single quotes, safely escaping any internal single quotes for shell execution.
func EscapeShellArg(arg string) string {
	return fmt.Sprintf("'%v'", strings.ReplaceAll(arg, "'", "'\\''"))
}

// RequireRealDirectory verifies that path exists and is a real directory.
// Symlinks are rejected because os.Lstat reports the symlink itself rather than
// its target, so a symlink to a directory does not satisfy FileInfo.IsDir.
func RequireRealDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("failed to access %v: %w", path, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("%v is not a directory", path)
	}

	return nil
}
