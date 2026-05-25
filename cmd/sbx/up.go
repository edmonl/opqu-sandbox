package main

import (
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/edmonl/opqu-sandbox/internal/config"
	"github.com/edmonl/opqu-sandbox/internal/sandbox"
	"github.com/edmonl/opqu-sandbox/internal/util"
	"github.com/spf13/cobra"
)

//go:embed nspawn.tmpl
var nspawnTemplate string

type nspawnTemplateData struct {
	Conf   *config.Config
	Mounts []*config.Mount
}

var upCmd = &cobra.Command{
	Use:     "up [name]",
	Aliases: []string{"u"},
	Short:   "Power on a sandbox",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if err := sandbox.ValidateName(name); err != nil {
			return err
		}

		if err := sandbox.Sudo(sbxDir); err != nil {
			return err
		}

		conf, err := config.LoadConf(sbxDir, name)
		if err != nil {
			return err
		}

		rootfsPath := filepath.Join(sbxDir, "rootfs", name)
		imageSymlink := filepath.Join(conf.ImagesPath, name)
		if ok, e := util.CheckSymlinkTarget(imageSymlink, rootfsPath); e != nil {
			return fmt.Errorf("invalid sandbox image symlink: %w", e)
		} else if !ok {
			return fmt.Errorf("sandbox image symlink %v does not point to rootfs %v", imageSymlink, rootfsPath)
		}

		if running, e := sandbox.IsRunning(name); e != nil {
			return e
		} else if running {
			util.Warn("sandbox %v is already running", name)
			return nil
		}

		mounts, err := config.LoadMounts(sbxDir, name, conf.SandboxUser)
		if err != nil {
			return err
		}

		if e := os.MkdirAll(conf.NspawnFilesPath, 0o755); e != nil {
			return fmt.Errorf("failed to create nspawn files directory %v: %w", conf.NspawnFilesPath, e)
		}

		nspawnPath, err := createNspawnFile(filepath.Join(sbxDir, "rootfs"), name, conf, mounts)
		if err != nil {
			return fmt.Errorf("failed to create nspawn file: %w", err)
		}

		if err := sandbox.CreateSymlink(nspawnPath, filepath.Join(conf.NspawnFilesPath, name+".nspawn")); err != nil {
			return fmt.Errorf("failed to create nspawn file symlink: %w", err)
		}

		if err := util.RunCmd("machinectl", "start", name); err != nil {
			return fmt.Errorf("failed to start sandbox %v using machinectl: %w", name, err)
		}

		if err := util.RunCmd("machinectl", "enable", name); err != nil {
			util.Warn("failed to enable sandbox %v: %v", name, err)
		}

		return nil
	},
}

func createNspawnFile(rootfsDir, name string, conf *config.Config, mounts []*config.Mount) (string, error) {
	nspawnPath := filepath.Join(rootfsDir, name+".nspawn")

	info, err := os.Lstat(nspawnPath)
	if err == nil {
		if info.Mode().IsRegular() {
			prompt := fmt.Sprintf("File %v already exists. Press <Enter> directly to overwrite, or Ctrl+C to cancel: ", nspawnPath)
			if confirmed, e := util.Confirm(prompt); e != nil {
				return "", e
			} else if !confirmed {
				return "", fmt.Errorf("user cancelled overwriting %v", nspawnPath)
			}
		} else {
			return "", fmt.Errorf("%v exists but is not a regular file", nspawnPath)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("failed to access %v: %w", nspawnPath, err)
	}

	tmpl, err := template.New("nspawn").Parse(nspawnTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse nspawn template: %w", err)
	}

	var sb strings.Builder
	if err := tmpl.Execute(&sb, nspawnTemplateData{Conf: conf, Mounts: mounts}); err != nil {
		return "", fmt.Errorf("failed to execute nspawn template: %w", err)
	}

	if err := os.WriteFile(nspawnPath, []byte(sb.String()), 0o644); err != nil {
		return "", fmt.Errorf("failed to write %v: %w", nspawnPath, err)
	}

	return nspawnPath, nil
}

func init() {
	rootCmd.AddCommand(upCmd)
}
