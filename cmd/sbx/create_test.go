package main

import (
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"

	"github.com/edmonl/opqu-sandbox/internal/config"
)

func TestGetSetupScript(t *testing.T) {
	conf := &config.Config{
		SandboxUser: &user.User{
			Username: "testuser",
			Uid:      "1000",
			Gid:      "1000",
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
		SandboxUser: &user.User{
			Username: "testuser",
			Uid:      "1000",
			Gid:      "1000",
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
		SandboxUser: &user.User{
			Username: "root",
			Uid:      "0",
			Gid:      "0",
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

func TestCreateDirCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	rootfsPath := filepath.Join(tmpDir, "rootfs")

	if _, err := createDir(tmpDir, "rootfs"); err != nil {
		t.Fatalf("createDir failed: %v", err)
	}
	if info, err := os.Stat(rootfsPath); err != nil {
		t.Fatalf("failed to stat rootfs directory: %v", err)
	} else if !info.IsDir() {
		t.Fatalf("rootfs path is not a directory")
	}
}

func TestCreateDirKeepsExistingDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	rootfsPath := filepath.Join(tmpDir, "rootfs")

	if err := os.Mkdir(rootfsPath, 0o755); err != nil {
		t.Fatalf("failed to create rootfs directory: %v", err)
	}
	if _, err := createDir(tmpDir, "rootfs"); err != nil {
		t.Fatalf("createDir rejected an existing directory: %v", err)
	}
}

func TestCreateDirAcceptsExistingSymlink(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "target")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatalf("failed to create target directory: %v", err)
	}

	rootfsPath := filepath.Join(tmpDir, "rootfs")

	if err := os.Symlink(target, rootfsPath); err != nil {
		t.Fatalf("failed to create rootfs symlink: %v", err)
	}
	if _, err := createDir(tmpDir, "rootfs"); err != nil {
		t.Fatalf("createDir rejected an existing symlink: %v", err)
	}
}

func TestCreateDirAcceptsSymlinkParent(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "target")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatalf("failed to create target directory: %v", err)
	}

	link := filepath.Join(tmpDir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("failed to create parent symlink: %v", err)
	}

	if _, err := createDir(tmpDir, "link", "child"); err != nil {
		t.Fatalf("createDir rejected a symlink parent: %v", err)
	}
	if info, err := os.Stat(filepath.Join(target, "child")); err != nil {
		t.Fatalf("failed to stat child through symlink parent: %v", err)
	} else if !info.IsDir() {
		t.Fatal("child path is not a directory")
	}
}
