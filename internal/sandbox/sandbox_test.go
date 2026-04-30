package sandbox

import (
	"testing"
)

func TestValidateName(t *testing.T) {
	validNames := []string{"test", "a", "123", "a-b", "test-sandbox", "this-is-longer-than-twelve-characters"}
	for _, name := range validNames {
		if err := ValidateName(name); err != nil {
			t.Errorf("Expected name %v to be valid, got error: %v", name, err)
		}
	}

	invalidNames := []string{
		"",          // too short
		"Test",      // uppercase
		"test_1",    // underscore
		"test name", // space
	}
	for _, name := range invalidNames {
		if err := ValidateName(name); err == nil {
			t.Errorf("Expected name %v to be invalid, but it was accepted", name)
		}
	}
}

func TestNames(t *testing.T) {
	name := "my-box"

	if MachineName(name) != "opqu-sbx-my-box" {
		t.Errorf("Unexpected machine name: %v", MachineName(name))
	}

	if BridgeName("test-zone") != "vz-test-zone" {
		t.Errorf("Unexpected bridge name: %v", BridgeName("test-zone"))
	}
}
