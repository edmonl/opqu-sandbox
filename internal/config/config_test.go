package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPackages(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.packages")

	content := `
# A comment
git

curl
# Another comment
ripgrep
`
	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	packages := loadPackages(testFile)

	if len(packages) != 3 {
		t.Fatalf("Expected 3 packages, got %d", len(packages))
	}

	if packages[0] != "git" || packages[1] != "curl" || packages[2] != "ripgrep" {
		t.Errorf("Unexpected packages: %v", packages)
	}
}

func TestLoadMounts(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.mounts")

	content := `
# Comment
/host/path:/sandbox/path
/another/host:/another/sandbox:ro
`
	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	mounts := loadMounts(testFile)

	if len(mounts) != 2 {
		t.Fatalf("Expected 2 mounts, got %d", len(mounts))
	}

	if mounts[0].HostPath != "/host/path" || mounts[0].SandboxPath != "/sandbox/path" || mounts[0].ReadOnly {
		t.Errorf("Unexpected first mount: %+v", mounts[0])
	}

	if mounts[1].HostPath != "/another/host" || mounts[1].SandboxPath != "/another/sandbox" || !mounts[1].ReadOnly {
		t.Errorf("Unexpected second mount: %+v", mounts[1])
	}
}

func TestLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	confDir := filepath.Join(tmpDir, "conf")
	os.MkdirAll(confDir, 0755)

	defaultConf := "DISTRO=bullseye\nAUDIO=yes\n"
	os.WriteFile(filepath.Join(confDir, "default"), []byte(defaultConf), 0644)

	sandboxConf := "PORTS=tcp:8080:80\n"
	os.WriteFile(filepath.Join(confDir, "test.conf"), []byte(sandboxConf), 0644)

	conf, err := LoadConf(tmpDir, "test")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if conf.Distro != "bullseye" {
		t.Errorf("Expected distro bullseye, got %s", conf.Distro)
	}

	if !conf.Audio {
		t.Errorf("Expected audio to be true")
	}

	if len(conf.Ports) != 1 || conf.Ports[0] != "tcp:8080:80" {
		t.Errorf("Expected ports [tcp:8080:80], got %v", conf.Ports)
	}
}

func TestInvalidPorts(t *testing.T) {
	tmpDir := t.TempDir()
	confDir := filepath.Join(tmpDir, "conf")
	os.MkdirAll(confDir, 0755)

	nameConf := "PORTS=invalid-port\n"
	os.WriteFile(filepath.Join(confDir, "test.conf"), []byte(nameConf), 0644)

	_, err := LoadConf(tmpDir, "test")
	if err == nil {
		t.Fatal("Expected error for invalid port mapping, but got none")
	}
}
