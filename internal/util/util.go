package util

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

func Confirm(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		if err == io.EOF {
			fmt.Println()
			return "^D", nil
		}
		return "", err
	}

	return strings.TrimRight(input, "\r\n"), nil
}

// EscapeShellArg wraps a string in single quotes, safely escaping any internal single quotes for shell execution.
func EscapeShellArg(arg string) string {
	return fmt.Sprintf("'%v'", strings.ReplaceAll(arg, "'", "'\\''"))
}
