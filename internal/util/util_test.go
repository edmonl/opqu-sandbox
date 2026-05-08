package util

import (
	"testing"
)

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
