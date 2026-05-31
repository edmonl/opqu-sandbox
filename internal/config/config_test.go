package config

import (
	"os"
	"os/user"
	"path/filepath"
	"testing"

	"github.com/edmonl/opqu-sandbox/internal/util"
)

func TestLoadPackages(t *testing.T) {
	tmpDir := t.TempDir()
	confDir := filepath.Join(tmpDir, "conf")
	os.MkdirAll(confDir, 0755)

	testFile := filepath.Join(confDir, "test.packages")

	content := `
# A comment
git

curl
# Another comment
ripgrep
git
`
	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	packages, err := LoadPackages(tmpDir, "test")
	if err != nil {
		t.Fatalf("LoadPackages failed: %v", err)
	}

	if len(packages) != 3 {
		t.Fatalf("Expected 3 unique packages, got %d", len(packages))
	}

	pkgMap := make(map[string]bool)
	for _, p := range packages {
		pkgMap[p] = true
	}

	if !pkgMap["git"] || !pkgMap["curl"] || !pkgMap["ripgrep"] {
		t.Errorf("Unexpected packages: %v", packages)
	}
}

func TestLoadMounts(t *testing.T) {
	tmpDir := t.TempDir()
	confDir := filepath.Join(tmpDir, "conf")
	os.MkdirAll(confDir, 0755)

	u, err := user.Current()
	if err != nil {
		t.Fatalf("Failed to get current user: %v", err)
	}
	testUser := &util.User{User: u}

	testFile := filepath.Join(confDir, "test.mounts")

	content := `
# Comment
/host/path:/sandbox/path
/another/host:/another/sandbox:ro
~/data:/home/user/data
relative/path:/abs/path
: /scratch
/host/path:/sandbox/path:ro
`
	err = os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	mounts, err := LoadMounts(tmpDir, "test", testUser)
	if err != nil {
		t.Fatalf("LoadMounts failed: %v", err)
	}

	if len(mounts) != 5 {
		t.Fatalf("Expected 5 mounts, got %d", len(mounts))
	}

	mountMap := make(map[string]*Mount)
	for _, m := range mounts {
		mountMap[m.SandboxPath] = m
	}

	// 1. /sandbox/path
	if m := mountMap["/sandbox/path"]; m == nil || m.HostPath != "/host/path" || !m.ReadOnly {
		t.Errorf("Unexpected mount for /sandbox/path: %+v (expected ReadOnly due to merge)", m)
	}

	// 2. /another/sandbox
	if m := mountMap["/another/sandbox"]; m == nil || m.HostPath != "/another/host" || !m.ReadOnly {
		t.Errorf("Unexpected mount for /another/sandbox: %+v", m)
	}

	// 3. /home/user/data
	expectedHomeData := filepath.Join(testUser.HomeDir, "data")
	if m := mountMap["/home/user/data"]; m == nil || m.HostPath != expectedHomeData {
		t.Errorf("Unexpected mount for /home/user/data: %+v", m)
	}

	// 4. /abs/path
	expectedRelPath := filepath.Join(tmpDir, "relative/path")
	if m := mountMap["/abs/path"]; m == nil || m.HostPath != expectedRelPath {
		t.Errorf("Unexpected mount for /abs/path: %+v", m)
	}

	// 5. /scratch
	if m := mountMap["/scratch"]; m == nil || m.HostPath != "" {
		t.Errorf("Unexpected mount for /scratch: %+v", m)
	}
}

func TestLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	confDir := filepath.Join(tmpDir, "conf")
	os.MkdirAll(confDir, 0755)

	defaultConf := "DISTRO=bullseye\nNETWORK_ZONE=test-zone\n"
	os.WriteFile(filepath.Join(confDir, "default"), []byte(defaultConf), 0644)

	sandboxConf := "PORTS=tcp:8080:80\n"
	os.WriteFile(filepath.Join(confDir, "test.conf"), []byte(sandboxConf), 0644)

	conf, err := LoadConf(tmpDir, "test")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if conf.Distro != "bullseye" {
		t.Errorf("Expected distro bullseye, got %v", conf.Distro)
	}

	if conf.NetworkZone != "test-zone" {
		t.Errorf("Expected network zone test-zone, got %v", conf.NetworkZone)
	}

	if len(conf.Ports) != 1 || conf.Ports[0] != "tcp:8080:80" {
		t.Errorf("Expected ports [tcp:8080:80], got %v", conf.Ports)
	}
}

func TestNetworkZoneLength(t *testing.T) {
	tmpDir := t.TempDir()
	confDir := filepath.Join(tmpDir, "conf")
	os.MkdirAll(confDir, 0755)

	nameConf := "NETWORK_ZONE=this-zone-is-too-long\n"
	os.WriteFile(filepath.Join(confDir, "test.conf"), []byte(nameConf), 0644)

	_, err := LoadConf(tmpDir, "test")
	if err == nil {
		t.Fatal("Expected error for long NETWORK_ZONE, but got none")
	}
}

func TestNetworkZoneCharacters(t *testing.T) {
	tmpDir := t.TempDir()
	confDir := filepath.Join(tmpDir, "conf")
	os.MkdirAll(confDir, 0755)

	invalidZones := []string{"test_zone", "test.zone", "Test-Zone", "test zone", "test/zone"}
	for _, zone := range invalidZones {
		nameConf := "NETWORK_ZONE=" + zone + "\n"
		os.WriteFile(filepath.Join(confDir, "test.conf"), []byte(nameConf), 0644)

		_, err := LoadConf(tmpDir, "test")
		if err == nil {
			t.Errorf("Expected error for invalid NETWORK_ZONE %v, but got none", zone)
		}
	}
}

func TestLoadConf_DefaultValues(t *testing.T) {
	tmpDir := t.TempDir()
	// No conf files created

	conf, err := LoadConf(tmpDir, "any")
	if err != nil {
		t.Fatalf("LoadConf failed: %v", err)
	}

	if conf.Distro != "stable" {
		t.Errorf("Expected default distro stable, got %v", conf.Distro)
	}
	if conf.NetworkZone != "opqu-sbx" {
		t.Errorf("Expected default network zone opqu-sbx, got %v", conf.NetworkZone)
	}
	currentUser, err := user.Current()
	if err != nil {
		t.Fatalf("user.Current failed: %v", err)
	}
	if conf.SandboxUser.Username != currentUser.Username {
		t.Errorf("Expected sandbox user %v, got %v", currentUser.Username, conf.SandboxUser.Username)
	}
}

func TestLoadConf_Override(t *testing.T) {
	tmpDir := t.TempDir()
	confDir := filepath.Join(tmpDir, "conf")
	os.MkdirAll(confDir, 0755)

	os.WriteFile(filepath.Join(confDir, "default"), []byte("DISTRO=bullseye\nNETWORK_ZONE=zone1"), 0644)
	os.WriteFile(filepath.Join(confDir, "test.conf"), []byte("NETWORK_ZONE=zone2"), 0644)

	conf, err := LoadConf(tmpDir, "test")
	if err != nil {
		t.Fatalf("LoadConf failed: %v", err)
	}

	if conf.Distro != "bullseye" {
		t.Errorf("Expected distro bullseye (from default), got %v", conf.Distro)
	}
	if conf.NetworkZone != "zone2" {
		t.Errorf("Expected network zone zone2 (from test.conf), got %v", conf.NetworkZone)
	}
}

func TestLoadPackages_Invalid(t *testing.T) {
	tmpDir := t.TempDir()
	confDir := filepath.Join(tmpDir, "conf")
	os.MkdirAll(confDir, 0755)

	os.WriteFile(filepath.Join(confDir, "test.packages"), []byte("invalid_package_name!"), 0644)

	_, err := LoadPackages(tmpDir, "test")
	if err == nil {
		t.Fatal("Expected error for invalid package name, but got none")
	}
}

func TestLoadMounts_EdgeCases(t *testing.T) {
	tmpDir := t.TempDir()
	confDir := filepath.Join(tmpDir, "conf")
	os.MkdirAll(confDir, 0755)

	u, err := user.Current()
	if err != nil {
		t.Fatalf("user.Current failed: %v", err)
	}
	testUser := &util.User{User: u}

	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{
			name:    "Invalid sandbox path (not absolute)",
			content: "/host:relative/path",
			wantErr: true,
		},
		{
			name:    "Empty both paths",
			content: ":",
			wantErr: true,
		},
		{
			name:    "Scratch mount",
			content: ": /scratch",
			wantErr: false,
		},
		{
			name:    "Conflicting host paths to same sandbox path",
			content: "/host1:/sandbox\n/host2:/sandbox",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.WriteFile(filepath.Join(confDir, "test.mounts"), []byte(tt.content), 0644)
			_, err := LoadMounts(tmpDir, "test", testUser)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadMounts() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
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
