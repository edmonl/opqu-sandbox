package main

import (
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"

	"github.com/edmonl/opqu-sandbox/internal/config"
	"github.com/edmonl/opqu-sandbox/internal/util"
)

func TestGetSetupScript(t *testing.T) {
	conf := &config.Config{
		SandboxUser: &util.User{
			User: &user.User{
				Username: "testuser",
				Uid:      "1000",
				Gid:      "1000",
			},
			UID: 1000,
			GID: 1000,
		},
		RootPassword: "rootpassword",
	}
	script := getSetupScript(conf)

	if !strings.HasPrefix(script, "set -e\n") {
		t.Errorf("script does not enable stop-on-error: %v", script)
	}
	if strings.Contains(script, " && ") {
		t.Errorf("script should use newline-separated commands: %v", script)
	}
	if strings.Contains(script, "/etc/hostname") || strings.Contains(script, "/etc/hosts") {
		t.Errorf("script should not write static files: %v", script)
	}
	if !strings.Contains(script, "groupadd -g 1000 'testuser'") {
		t.Errorf("script does not contain primary group creation: %v", script)
	}
	if !strings.Contains(script, "useradd -m -u 1000 -g 1000 -s /bin/bash 'testuser'") {
		t.Errorf("script does not contain user creation with primary group: %v", script)
	}
	if !strings.Contains(script, "printf '%s\\n' 'root:''rootpassword' | chpasswd") {
		t.Errorf("script does not contain root password setup: %v", script)
	}
}

func TestGetSetupScriptNoRootPassword(t *testing.T) {
	conf := &config.Config{
		SandboxUser: &util.User{
			User: &user.User{
				Username: "testuser",
				Uid:      "1000",
				Gid:      "1000",
			},
			UID: 1000,
			GID: 1000,
		},
		RootPassword: "",
	}
	script := getSetupScript(conf)

	if !strings.Contains(script, "passwd -l root") {
		t.Errorf("script does not contain root locking: %v", script)
	}
}

func TestGetSetupScriptRootUser(t *testing.T) {
	conf := &config.Config{
		SandboxUser: &util.User{
			User: &user.User{
				Username: "root",
				Uid:      "0",
				Gid:      "0",
			},
			UID: 0,
			GID: 0,
		},
		RootPassword: "rootpassword",
	}
	script := getSetupScript(conf)

	if strings.Contains(script, "useradd") {
		t.Errorf("script should not contain useradd for root user: %v", script)
	}
	if !strings.Contains(script, "printf '%s\\n' 'root:''rootpassword' | chpasswd") {
		t.Errorf("script should contain chpasswd for root user: %v", script)
	}
}

func TestWriteSandboxFiles(t *testing.T) {
	rootfsPath := t.TempDir()
	etcPath := filepath.Join(rootfsPath, "etc")
	if err := os.Mkdir(etcPath, 0o755); err != nil {
		t.Fatalf("failed to create etc directory: %v", err)
	}

	if err := writeSandboxFiles(rootfsPath, "mysandbox"); err != nil {
		t.Fatalf("writeSandboxFiles failed: %v", err)
	}

	hostname, err := os.ReadFile(filepath.Join(etcPath, "hostname"))
	if err != nil {
		t.Fatalf("failed to read hostname: %v", err)
	}
	if string(hostname) != "mysandbox\n" {
		t.Errorf("hostname = %q, want %q", string(hostname), "mysandbox\n")
	}

	hosts, err := os.ReadFile(filepath.Join(etcPath, "hosts"))
	if err != nil {
		t.Fatalf("failed to read hosts: %v", err)
	}
	if !strings.Contains(string(hosts), "127.0.1.1\tmysandbox\n") {
		t.Errorf("hosts does not contain sandbox hostname: %q", string(hosts))
	}
}
