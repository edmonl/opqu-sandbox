// Package util provides general utility functions used across the application.
package util

import (
	"bufio"
	"fmt"
	"io"
	"os"
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
