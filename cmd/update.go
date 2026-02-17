package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/MagicLex/hopsworks-cli/pkg/output"
	"github.com/spf13/cobra"
)

const githubRepo = "MagicLex/hopsworks-cli"

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update hops to the latest version",
	RunE: func(cmd *cobra.Command, args []string) error {
		output.Info("Current version: %s", Version)

		// Check latest version from GitHub
		latest, err := getLatestVersion()
		if err != nil {
			return fmt.Errorf("check latest version: %w", err)
		}

		if latest == Version || latest == "v"+Version {
			output.Success("Already up to date")
			return nil
		}

		output.Info("Latest version: %s", latest)

		// Find where hops is installed
		currentBin, err := os.Executable()
		if err != nil {
			return fmt.Errorf("find current binary: %w", err)
		}
		currentBin, _ = filepath.EvalSymlinks(currentBin)

		// Build from source via go install
		if _, err := exec.LookPath("go"); err != nil {
			return fmt.Errorf("go not found in PATH — install Go or update manually")
		}

		output.Info("Building latest from source...")

		tmpDir, err := os.MkdirTemp("", "hops-update-*")
		if err != nil {
			return fmt.Errorf("create temp dir: %w", err)
		}
		defer os.RemoveAll(tmpDir)

		goInstall := exec.Command("go", "install",
			"-ldflags", fmt.Sprintf("-X github.com/%s/cmd.Version=%s", githubRepo, strings.TrimPrefix(latest, "v")),
			fmt.Sprintf("github.com/%s@%s", githubRepo, latest),
		)
		goInstall.Stdout = os.Stdout
		goInstall.Stderr = os.Stderr
		if err := goInstall.Run(); err != nil {
			return fmt.Errorf("go install failed: %w", err)
		}

		// Find the built binary in GOPATH/bin
		gopath := os.Getenv("GOPATH")
		if gopath == "" {
			home, _ := os.UserHomeDir()
			gopath = filepath.Join(home, "go")
		}
		builtBin := filepath.Join(gopath, "bin", "hopsworks-cli")

		if _, err := os.Stat(builtBin); err != nil {
			return fmt.Errorf("built binary not found at %s", builtBin)
		}

		// Copy to current location
		if err := copyFile(builtBin, currentBin); err != nil {
			// Try the go/bin rename as fallback (same filesystem)
			target := filepath.Join(filepath.Dir(builtBin), "hops")
			if err2 := os.Rename(builtBin, target); err2 != nil {
				return fmt.Errorf("install failed: %w (also tried rename: %v)", err, err2)
			}
			output.Success("Updated to %s at %s", latest, target)
			return nil
		}

		// Clean up the hopsworks-cli name in go/bin
		os.Remove(builtBin)

		output.Success("Updated to %s", latest)
		return nil
	},
}

func getLatestVersion() (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	// Try releases first
	resp, err := client.Get(fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", githubRepo))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		var release struct {
			TagName string `json:"tag_name"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&release); err == nil && release.TagName != "" {
			return release.TagName, nil
		}
	}

	// Fallback: latest tag
	resp2, err := client.Get(fmt.Sprintf("https://api.github.com/repos/%s/tags?per_page=1", githubRepo))
	if err != nil {
		return "", err
	}
	defer resp2.Body.Close()

	var tags []struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp2.Body).Decode(&tags); err != nil {
		return "", err
	}
	if len(tags) > 0 {
		return tags[0].Name, nil
	}

	return "", fmt.Errorf("no releases or tags found on %s", githubRepo)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
