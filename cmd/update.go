package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/MagicLex/hopsworks-cli/pkg/output"
	"github.com/spf13/cobra"
)

const githubRepo = "MagicLex/hopsworks-cli"

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Check for updates and improvements",
	RunE: func(cmd *cobra.Command, args []string) error {
		output.Info("Current version: %s", Version)

		latest, err := getLatestVersion()
		if err != nil {
			return fmt.Errorf("check latest version: %w", err)
		}

		if latest == Version || latest == "v"+Version {
			output.Success("Already up to date")
			return nil
		}

		output.Info("Latest version: %s", latest)

		currentBin, err := os.Executable()
		if err != nil {
			return fmt.Errorf("find current binary: %w", err)
		}
		currentBin, _ = filepath.EvalSymlinks(currentBin)

		// Download pre-built binary from GitHub release
		assetName := fmt.Sprintf("hops-%s-%s", runtime.GOOS, runtime.GOARCH)
		downloadURL := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", githubRepo, latest, assetName)

		output.Info("Downloading %s...", downloadURL)

		tmpFile, err := os.CreateTemp("", "hops-update-*")
		if err != nil {
			return fmt.Errorf("create temp file: %w", err)
		}
		tmpPath := tmpFile.Name()
		defer os.Remove(tmpPath)

		client := &http.Client{Timeout: 60 * time.Second}
		resp, err := client.Get(downloadURL)
		if err != nil {
			return fmt.Errorf("download failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			return fmt.Errorf("download failed: HTTP %d (asset %s may not exist for this platform)", resp.StatusCode, assetName)
		}

		if _, err := io.Copy(tmpFile, resp.Body); err != nil {
			tmpFile.Close()
			return fmt.Errorf("download failed: %w", err)
		}
		tmpFile.Close()

		if err := os.Chmod(tmpPath, 0755); err != nil {
			return fmt.Errorf("chmod failed: %w", err)
		}

		if err := copyFile(tmpPath, currentBin); err != nil {
			return fmt.Errorf("replace binary failed: %w (try: curl -L %s -o %s)", err, downloadURL, currentBin)
		}

		output.Success("Updated to %s", latest)
		return nil
	},
}

func getLatestVersion() (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}

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
