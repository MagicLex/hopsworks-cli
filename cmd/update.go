package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/MagicLex/hopsworks-cli/pkg/output"
	"github.com/spf13/cobra"
)

const moduleRepo = "github.com/MagicLex/hopsworks-cli"

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Build and install the latest version from source",
	RunE: func(cmd *cobra.Command, args []string) error {
		output.Info("Current version: %s", Version)

		currentBin, err := os.Executable()
		if err != nil {
			return fmt.Errorf("find current binary: %w", err)
		}
		currentBin, _ = filepath.EvalSymlinks(currentBin)

		output.Info("Installing latest from %s...", moduleRepo)

		goInstall := exec.Command("go", "install", moduleRepo+"@latest")
		goInstall.Stdout = os.Stdout
		goInstall.Stderr = os.Stderr
		if err := goInstall.Run(); err != nil {
			return fmt.Errorf("go install failed: %w", err)
		}

		// go install puts the binary as "hopsworks-cli" in GOPATH/bin
		gopath, err := goEnv("GOPATH")
		if err != nil {
			return fmt.Errorf("find GOPATH: %w", err)
		}
		installed := filepath.Join(gopath, "bin", "hopsworks-cli")

		if currentBin != installed {
			if err := copyFile(installed, currentBin); err != nil {
				return fmt.Errorf("copy to %s: %w", currentBin, err)
			}
			output.Success("Updated %s", currentBin)
		} else {
			output.Success("Updated (binary at %s)", installed)
		}

		return nil
	},
}

func goEnv(key string) (string, error) {
	out, err := exec.Command("go", "env", key).Output()
	if err != nil {
		return "", err
	}
	val := string(out)
	// Trim trailing newline
	if len(val) > 0 && val[len(val)-1] == '\n' {
		val = val[:len(val)-1]
	}
	return val, nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	// Remove destination first to avoid "text file busy" when overwriting a running binary.
	// Linux allows unlinking open files — the old inode stays alive for the running process.
	_ = os.Remove(dst)
	return os.WriteFile(dst, data, 0755)
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
