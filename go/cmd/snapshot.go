package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/edmonl/opqu-sandbox/internal/sandbox"
	"github.com/spf13/cobra"
)

var snapshotCmd = &cobra.Command{
	Use:   "snapshot [name] [output_path]",
	Short: "Save a stopped sandbox rootfs to a tar.zst archive",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if err := sandbox.ValidateName(name); err != nil {
			return err
		}

		running, err := sandbox.IsRunning(name)
		if err != nil {
			return err
		}

		if running {
			return fmt.Errorf("sandbox '%s' is running; stop it first", name)
		}

		var outputPath string
		if len(args) == 2 {
			outputPath = args[1]
		} else {
			cwd, _ := os.Getwd()
			outputPath = filepath.Join(cwd, fmt.Sprintf("opqu-sbx-%s.snapshot.tar.zst", name))
		}

		if _, err := os.Stat(outputPath); err == nil {
			return fmt.Errorf("snapshot output '%s' already exists; move it away first", outputPath)
		}

		outputDir := filepath.Dir(outputPath)
		outputBase := filepath.Base(outputPath)

		tmpFile, err := os.CreateTemp(outputDir, fmt.Sprintf(".%s.tmp.*", outputBase))
		if err != nil {
			return fmt.Errorf("failed to create temporary snapshot file in '%s': %v", outputDir, err)
		}
		tmpPath := tmpFile.Name()
		tmpFile.Close() // Close it since we'll redirect stdout of tar to it

		// tar -I 'zstd -19 -T0' -cf - -C "$ROOT_DIR" "rootfs-$name/" > tmp_output
		tarCmd := exec.Command("tar", "-I", "zstd -19 -T0", "-cf", "-", "-C", rootDir, "rootfs-"+name+"/")
		outFile, err := os.OpenFile(tmpPath, os.O_WRONLY, 0644)
		if err != nil {
			os.Remove(tmpPath)
			return err
		}
		defer outFile.Close()

		tarCmd.Stdout = outFile
		tarCmd.Stderr = os.Stderr

		if err := tarCmd.Run(); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("failed to create snapshot: %v", err)
		}

		if err := os.Rename(tmpPath, outputPath); err != nil {
			return fmt.Errorf("failed to move snapshot into place at '%s'; temporary snapshot kept at '%s': %v", outputPath, tmpPath, err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(snapshotCmd)
}
