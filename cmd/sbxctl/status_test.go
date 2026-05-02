package main

import (
	"testing"
)

func TestGetCommandStatus(t *testing.T) {
	// Test with a command that is almost certainly present on any Linux system
	if status := getCommandStatus("ls"); status != "ok" {
		t.Errorf("getCommandStatus(\"ls\") = %v, want \"ok\"", status)
	}

	// Test with a command that is almost certainly not present
	if status := getCommandStatus("non-existent-command-12345"); status != "not available" {
		t.Errorf("getCommandStatus(\"non-existent-command\") = %v, want \"not available\"", status)
	}
}
