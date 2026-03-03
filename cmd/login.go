package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/MagicLex/hopsworks-cli/pkg/client"
	"github.com/MagicLex/hopsworks-cli/pkg/output"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const defaultHost = "https://eu-west.cloud.hopsworks.ai"

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with Hopsworks",
	Long:  `Login to a Hopsworks instance. Validates the API key and saves config to ~/.hops/config.`,
	RunE: loginRun,
}

func loginRun(cmd *cobra.Command, args []string) error {
	if cfg.Internal {
		output.Success("Internal mode — already authenticated via JWT")
		output.Info("  Host: %s", cfg.Host)
		output.Info("  Project: %s", cfg.Project)
		return nil
	}

	reader := bufio.NewReader(os.Stdin)

	// Prompt for host if not set
	if cfg.Host == "" {
		host, err := promptHost(reader)
		if err != nil {
			return err
		}
		cfg.Host = host
	}

	// Prompt for API key if not set
	if cfg.APIKey == "" {
		apiKey, err := promptAPIKey(reader, cfg.Host)
		if err != nil {
			return err
		}
		cfg.APIKey = apiKey
	}

	// Validate by fetching projects
	c, err := client.New(cfg)
	if err != nil {
		return err
	}

	projects, err := c.ListProjects()
	if err != nil {
		return fmt.Errorf("authenticate: %w", err)
	}

	output.Success("Logged in to %s (%d projects accessible)", cfg.Host, len(projects))

	// Project selection
	if cfg.Project == "" && len(projects) > 0 {
		if len(projects) == 1 {
			cfg.Project = projects[0].ProjectName
			cfg.ProjectID = projects[0].ProjectID
			output.Success("Auto-selected project: %s", cfg.Project)
		} else {
			selected, err := promptProject(reader, projects)
			if err != nil {
				return err
			}
			cfg.Project = selected.ProjectName
			cfg.ProjectID = selected.ProjectID
			output.Success("Selected project: %s", cfg.Project)
		}

		if err := resolveFeatureStoreID(c); err == nil {
			// resolved, will be saved below
		}
	}

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	return nil
}

func promptHost(reader *bufio.Reader) (string, error) {
	fmt.Println()
	fmt.Println("  Select Hopsworks instance:")
	fmt.Printf("  [1] %s\n", defaultHost)
	fmt.Println("  [2] Custom URL")
	fmt.Print("\n  > ")

	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)

	switch choice {
	case "1", "":
		return defaultHost, nil
	case "2":
		fmt.Print("  Host URL: ")
		host, _ := reader.ReadString('\n')
		host = normalizeHost(strings.TrimSpace(host))
		if host == "" {
			return "", fmt.Errorf("host URL is required")
		}
		return host, nil
	default:
		// Treat as direct URL input
		return normalizeHost(choice), nil
	}
}

func normalizeHost(host string) string {
	host = strings.TrimRight(host, "/")
	if host != "" && !strings.Contains(host, "://") {
		host = "https://" + host
	}
	return host
}

func promptAPIKey(reader *bufio.Reader, host string) (string, error) {
	// Build the API keys page URL
	apiKeysURL := strings.TrimRight(host, "/") + "/account/api/generated"

	fmt.Println()
	output.Info("Opening browser to create/copy your API key...")
	fmt.Printf("  %s\n\n", apiKeysURL)

	openBrowser(apiKeysURL)

	fmt.Print("  API key: ")
	// Mask input if we're in a terminal
	if term.IsTerminal(int(os.Stdin.Fd())) {
		keyBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println() // newline after masked input
		if err != nil {
			return "", fmt.Errorf("read API key: %w", err)
		}
		key := strings.TrimSpace(string(keyBytes))
		if key == "" {
			return "", fmt.Errorf("API key is required")
		}
		return key, nil
	}

	// Non-terminal fallback (piped input)
	key, _ := reader.ReadString('\n')
	key = strings.TrimSpace(key)
	if key == "" {
		return "", fmt.Errorf("API key is required")
	}
	return key, nil
}

func promptProject(reader *bufio.Reader, projects []client.Project) (client.Project, error) {
	fmt.Println()
	fmt.Println("  Select project:")
	for i, p := range projects {
		fmt.Printf("  [%d] %s\n", i+1, p.ProjectName)
	}
	fmt.Print("\n  > ")

	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)

	idx, err := strconv.Atoi(choice)
	if err != nil || idx < 1 || idx > len(projects) {
		return projects[0], fmt.Errorf("invalid selection: %s", choice)
	}

	return projects[idx-1], nil
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return
	}
	// Best-effort — don't fail if browser can't open (SSH, headless)
	cmd.Start()
}

func resolveFeatureStoreID(c *client.Client) error {
	data, err := c.Get(fmt.Sprintf("/hopsworks-api/api/project/%d/featurestores", cfg.ProjectID))
	if err != nil {
		return err
	}
	var stores []struct {
		FeaturestoreId int    `json:"featurestoreId"`
		Name           string `json:"featurestoreName"`
	}
	if err := json.Unmarshal(data, &stores); err != nil {
		return err
	}
	for _, s := range stores {
		if strings.Contains(s.Name, cfg.Project) || len(stores) == 1 {
			cfg.FeatureStoreID = s.FeaturestoreId
			return nil
		}
	}
	if len(stores) > 0 {
		cfg.FeatureStoreID = stores[0].FeaturestoreId
	}
	return nil
}

func init() {
	rootCmd.AddCommand(loginCmd)
}
