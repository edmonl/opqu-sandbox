package util

import (
	"io"
	"os"
	"testing"
)

func TestWarn(t *testing.T) {
	oldStderr := os.Stderr
	readEnd, writeEnd, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stderr = writeEnd
	defer func() {
		os.Stderr = oldStderr
		readEnd.Close()
		writeEnd.Close()
	}()

	Warn("failed to do %v: %v", "thing", "reason")
	if err := writeEnd.Close(); err != nil {
		t.Fatalf("failed to close write end: %v", err)
	}

	output, err := io.ReadAll(readEnd)
	if err != nil {
		t.Fatalf("failed to read warning output: %v", err)
	}
	if got, want := string(output), "Warning: failed to do thing: reason\n"; got != want {
		t.Fatalf("Warn output = %q, want %q", got, want)
	}
}

func TestEscapeShellArg(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Simple string",
			input:    "hello",
			expected: "'hello'",
		},
		{
			name:     "String with spaces",
			input:    "hello world",
			expected: "'hello world'",
		},
		{
			name:     "String with single quote",
			input:    "don't",
			expected: "'don'\\''t'",
		},
		{
			name:     "String with multiple single quotes",
			input:    "o'reilly's",
			expected: "'o'\\''reilly'\\''s'",
		},
		{
			name:     "String with double quotes",
			input:    `said "hello"`,
			expected: "'said \"hello\"'",
		},
		{
			name:     "String with shell variables",
			input:    "$HOME",
			expected: "'$HOME'",
		},
		{
			name:     "String with command substitution",
			input:    "$(rm -rf /)",
			expected: "'$(rm -rf /)'",
		},
		{
			name:     "String with backslashes",
			input:    "C:\\Path",
			expected: "'C:\\Path'",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "''",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := EscapeShellArg(tt.input)
			if actual != tt.expected {
				t.Errorf("EscapeShellArg(%v) = %v, expected %v", tt.input, actual, tt.expected)
			}
		})
	}
}
