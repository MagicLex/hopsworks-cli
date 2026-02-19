package client

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/MagicLex/hopsworks-cli/pkg/config"
)

type Client struct {
	Config     *config.Config
	HTTPClient *http.Client
}

func New(cfg *config.Config) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	// Skip TLS verification for internal cluster communication
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	return &Client{
		Config: cfg,
		HTTPClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
	}, nil
}

func (c *Client) baseURL() string {
	host := strings.TrimRight(c.Config.Host, "/")
	if !strings.HasPrefix(host, "http") {
		host = "https://" + host
	}
	return host
}

func (c *Client) doRequest(method, path string, body io.Reader) ([]byte, error) {
	url := c.baseURL() + path

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Use JWT token if available (in-platform), otherwise API key
	if c.Config.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.Config.JWTToken)
	} else {
		req.Header.Set("Authorization", "ApiKey "+c.Config.APIKey)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		// Try to extract error message from JSON response
		var errResp struct {
			ErrorMsg string `json:"errorMsg"`
			UsrMsg   string `json:"usrMsg"`
			DevMsg   string `json:"devMsg"`
		}
		if json.Unmarshal(data, &errResp) == nil {
			msg := errResp.UsrMsg
			if msg == "" {
				msg = errResp.ErrorMsg
			}
			// Append devMsg when it has extra detail (e.g. Snowflake auth failures)
			if msg != "" && errResp.DevMsg != "" && errResp.DevMsg != msg {
				return nil, fmt.Errorf("API error (%d): %s — %s", resp.StatusCode, msg, errResp.DevMsg)
			}
			if msg != "" {
				return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, msg)
			}
		}
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(data))
	}

	return data, nil
}

func (c *Client) Get(path string) ([]byte, error) {
	return c.doRequest("GET", path, nil)
}

func (c *Client) Post(path string, body io.Reader) ([]byte, error) {
	return c.doRequest("POST", path, body)
}

func (c *Client) Put(path string, body io.Reader) ([]byte, error) {
	return c.doRequest("PUT", path, body)
}

func (c *Client) Delete(path string) ([]byte, error) {
	return c.doRequest("DELETE", path, nil)
}

// ProjectPath returns the base API path for the current project
func (c *Client) ProjectPath() string {
	return fmt.Sprintf("/hopsworks-api/api/project/%d", c.Config.ProjectID)
}

// FSPath returns the base API path for the feature store
func (c *Client) FSPath() string {
	return fmt.Sprintf("%s/featurestores/%d", c.ProjectPath(), c.Config.FeatureStoreID)
}
