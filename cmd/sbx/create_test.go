package main

import (
	"os/user"
	"strings"
	"testing"

	"github.com/edmonl/opqu-sandbox/internal/config"
)

func TestGetSetupScript(t *testing.T) {
	conf := &config.Config{
		SandboxUser: &user.User{
			Username: "testuser",
			Uid:      "1000",
		},
		RootPassword: "rootpassword",
	}
	name := "mysandbox"
	script := getSetupScript(name, conf)

	if !strings.Contains(script, "echo 'mysandbox' > /etc/hostname") {
		t.Errorf("script does not contain hostname setup: %v", script)
	}
	if !strings.Contains(script, "127.0.1.1\\t%s") || !strings.Contains(script, "'mysandbox' > /etc/hosts") {
		t.Errorf("script does not contain hosts setup: %v", script)
	}
	if !strings.Contains(script, "useradd -m -u 1000 -s /bin/bash 'testuser'") {
		t.Errorf("script does not contain user creation: %v", script)
	}
	if !strings.Contains(script, "echo 'root:''rootpassword' | chpasswd") {
		t.Errorf("script does not contain root password setup: %v", script)
	}
}

func TestGetSetupScriptNoRootPassword(t *testing.T) {
	conf := &config.Config{
		SandboxUser: &user.User{
			Username: "testuser",
			Uid:      "1000",
		},
		RootPassword: "",
	}
	name := "mysandbox"
	script := getSetupScript(name, conf)

	if !strings.Contains(script, "passwd -l root") {
		t.Errorf("script does not contain root locking: %v", script)
	}
}

func TestGetSetupScriptRootUser(t *testing.T) {
	conf := &config.Config{
		SandboxUser: &user.User{
			Username: "root",
			Uid:      "0",
		},
		RootPassword: "rootpassword",
	}
	name := "mysandbox"
	script := getSetupScript(name, conf)

	if strings.Contains(script, "useradd") {
		t.Errorf("script should not contain useradd for root user: %v", script)
	}
	if !strings.Contains(script, "echo 'root:''rootpassword' | chpasswd") {
		t.Errorf("script should contain chpasswd for root user: %v", script)
	}
}
