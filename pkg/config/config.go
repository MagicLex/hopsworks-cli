package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Host           string `yaml:"host"`
	APIKey         string `yaml:"api_key"`
	JWTToken       string `yaml:"-"` // Never persisted, loaded from env/file
	Project        string `yaml:"project"`
	ProjectID      int    `yaml:"project_id"`
	FeatureStoreID int    `yaml:"feature_store_id"`
}

func ConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".hops")
}

func ConfigPath() string {
	return filepath.Join(ConfigDir(), "config")
}

func Load() (*Config, error) {
	cfg := &Config{}

	// Environment variables override everything (in-platform detection)
	if endpoint := os.Getenv("REST_ENDPOINT"); endpoint != "" {
		cfg.Host = endpoint
	}
	if apiKey := os.Getenv("HOPSWORKS_API_KEY"); apiKey != "" {
		cfg.APIKey = apiKey
	}
	if project := os.Getenv("PROJECT_NAME"); project != "" {
		cfg.Project = project
	}
	if projectID := os.Getenv("HOPSWORKS_PROJECT_ID"); projectID != "" {
		if id, err := strconv.Atoi(projectID); err == nil {
			cfg.ProjectID = id
		}
	}

	// Load JWT token from secrets directory (in-platform)
	if secretsDir := os.Getenv("SECRETS_DIR"); secretsDir != "" {
		tokenPath := filepath.Join(secretsDir, "token.jwt")
		if tokenData, err := os.ReadFile(tokenPath); err == nil {
			cfg.JWTToken = strings.TrimSpace(string(tokenData))
		}
	}

	// Load config file (values don't override env vars)
	data, err := os.ReadFile(ConfigPath())
	if err == nil {
		var fileCfg Config
		if err := yaml.Unmarshal(data, &fileCfg); err == nil {
			if cfg.Host == "" {
				cfg.Host = fileCfg.Host
			}
			if cfg.APIKey == "" {
				cfg.APIKey = fileCfg.APIKey
			}
			if cfg.Project == "" {
				cfg.Project = fileCfg.Project
			}
			if cfg.ProjectID == 0 {
				cfg.ProjectID = fileCfg.ProjectID
			}
			if cfg.FeatureStoreID == 0 {
				cfg.FeatureStoreID = fileCfg.FeatureStoreID
			}
		}
	}

	return cfg, nil
}

func (c *Config) Save() error {
	if err := os.MkdirAll(ConfigDir(), 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.WriteFile(ConfigPath(), data, 0600)
}

func (c *Config) Validate() error {
	if c.Host == "" {
		return fmt.Errorf("no host configured. Run 'hops login' or set REST_ENDPOINT")
	}
	if c.APIKey == "" && c.JWTToken == "" {
		return fmt.Errorf("no API key or JWT token configured. Run 'hops login' or set HOPSWORKS_API_KEY")
	}
	return nil
}
