package main

import (
	_ "embed"
	"fmt"
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
	Use:   "up [name]",
	Short: "Power on a sandbox",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if err := sandbox.ValidateName(name); err != nil {
			return err
		}

		if err := sandbox.Sudo(sbxDir); err != nil {
			return err
		}

		if running, err := sandbox.IsRunning(name); err != nil {
			return err
		} else if running {
			fmt.Fprintf(os.Stderr, "Warning: sandbox %v is already running\n", name)
		} else {
			conf, err := config.LoadConf(sbxDir, name)
			if err != nil {
				return err
			}

			mounts, err := config.LoadMounts(sbxDir, name, conf.SandboxUser)
			if err != nil {
				return err
			}

			tmpl, err := template.New("nspawn").Parse(nspawnTemplate)
			if err != nil {
				return fmt.Errorf("failed to parse nspawn template: %w", err)
			}

			var sb strings.Builder
			if err := tmpl.Execute(&sb, nspawnTemplateData{Conf: conf, Mounts: mounts}); err != nil {
				return fmt.Errorf("failed to execute nspawn template: %w", err)
			}

			nspawnContent := sb.String()
			nspawnPath := filepath.Join(conf.ImagePath, name+".nspawn")

			if _, err := os.Stat(nspawnPath); err == nil {
				prompt := fmt.Sprintf("File %v already exists. Press <Enter> directly to overwrite, or Ctrl+C to cancel: ", nspawnPath)
				input, err := util.Confirm(prompt)
				if err != nil {
					return err
				}
				if input != "" {
					return fmt.Errorf("user cancelled overwriting %v", nspawnPath)
				}
			}

			removeNspawnFile := func() {
				if err := os.RemoveAll(nspawnPath); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to remove nspawn file %v: %v\n", nspawnPath, err)
				}
			}

			if err := os.WriteFile(nspawnPath, []byte(nspawnContent), 0o644); err != nil {
				removeNspawnFile()
				return fmt.Errorf("failed to create nspawn file %v: %w", nspawnPath, err)
			}

			if err := sandbox.RunCmd("machinectl", "start", name); err != nil {
				removeNspawnFile()
				return fmt.Errorf("failed to start sandbox %v using machinectl: %w", name, err)
			}
		}

		if err := sandbox.RunCmd("machinectl", "enable", name); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to enable sandbox %v: %v\n", name, err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(upCmd)
}
