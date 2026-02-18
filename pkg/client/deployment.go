package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Deployment struct {
	ID                 int    `json:"id,omitempty"`
	Name               string `json:"name"`
	ModelName          string `json:"modelName,omitempty"`
	ModelVersion       int    `json:"modelVersion,omitempty"`
	ModelPath          string `json:"modelPath,omitempty"`
	ModelServer        string `json:"modelServer,omitempty"`
	ModelFramework     string `json:"modelFramework,omitempty"`
	ServingTool        string `json:"servingTool,omitempty"`
	Predictor          string `json:"predictor,omitempty"`
	Status             string `json:"status,omitempty"`
	Creator            string `json:"creator,omitempty"`
	Created            interface{} `json:"created,omitempty"`
	RequestedInstances int    `json:"requestedInstances,omitempty"`
	APIProtocol        string `json:"apiProtocol,omitempty"`
	InferenceLogging   string `json:"inferenceLogging,omitempty"`
}

type DeploymentCreateRequest struct {
	Name               string                  `json:"name"`
	ModelServer        string                  `json:"modelServer"`
	ServingTool        string                  `json:"servingTool"`
	ModelName          string                  `json:"modelName"`
	ModelPath          string                  `json:"modelPath"`
	ModelVersion       int                     `json:"modelVersion"`
	ModelFramework     string                  `json:"modelFramework"`
	Predictor          string                  `json:"predictor,omitempty"`
	RequestedInstances int                     `json:"requestedInstances"`
	APIProtocol        string                  `json:"apiProtocol"`
	InferenceLogging   string                  `json:"inferenceLogging"`
	BatchingConfig     *DeployBatchingConfig   `json:"batchingConfiguration,omitempty"`
	PredictorResources *DeployResourceConfig   `json:"predictorResources,omitempty"`
}

type DeployBatchingConfig struct {
	BatchingEnabled bool `json:"batchingEnabled"`
}

type DeployResourceConfig struct {
	Requests DeployResources `json:"requests"`
	Limits   DeployResources `json:"limits"`
}

type DeployResources struct {
	Cores  float64 `json:"cores"`
	Memory int     `json:"memory"`
	GPUs   int     `json:"gpus"`
}

func (c *Client) ServingPath() string {
	return fmt.Sprintf("%s/serving", c.ProjectPath())
}

func (c *Client) ListDeployments() ([]Deployment, error) {
	data, err := c.Get(c.ServingPath())
	if err != nil {
		return nil, err
	}

	var deployments []Deployment
	if err := json.Unmarshal(data, &deployments); err != nil {
		return nil, fmt.Errorf("parse deployments: %w", err)
	}
	return deployments, nil
}

func (c *Client) GetDeployment(id int) (*Deployment, error) {
	data, err := c.Get(fmt.Sprintf("%s/%d", c.ServingPath(), id))
	if err != nil {
		return nil, err
	}

	var d Deployment
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("parse deployment: %w", err)
	}
	return &d, nil
}

func (c *Client) GetDeploymentByName(name string) (*Deployment, error) {
	deployments, err := c.ListDeployments()
	if err != nil {
		return nil, err
	}
	for i := range deployments {
		if deployments[i].Name == name {
			return &deployments[i], nil
		}
	}
	return nil, fmt.Errorf("deployment %q not found", name)
}

func (c *Client) CreateDeployment(req *DeploymentCreateRequest) (*Deployment, error) {
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal deployment: %w", err)
	}
	data, err := c.Put(c.ServingPath(), bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	var d Deployment
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("parse deployment response: %w", err)
	}
	return &d, nil
}

func (c *Client) DeploymentAction(id int, action string) error {
	path := fmt.Sprintf("%s/%d?action=%s", c.ServingPath(), id, action)
	_, err := c.Post(path, nil)
	return err
}

func (c *Client) DeleteDeployment(id int) error {
	_, err := c.Delete(fmt.Sprintf("%s/%d", c.ServingPath(), id))
	return err
}

func (c *Client) DeploymentLogs(id int, component string, tail int) (string, error) {
	path := fmt.Sprintf("%s/%d/logs?component=%s&tail=%d", c.ServingPath(), id, component, tail)
	data, err := c.Get(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (c *Client) Predict(modelName string, payload json.RawMessage) (json.RawMessage, error) {
	path := fmt.Sprintf("%s/inference/models/%s:predict", c.ProjectPath(), modelName)

	// Build request body
	body := map[string]json.RawMessage{"instances": payload}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal predict body: %w", err)
	}

	// Use doRequest directly to get raw response
	url := c.baseURL() + path
	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if c.Config.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.Config.JWTToken)
	} else {
		req.Header.Set("Authorization", "ApiKey "+c.Config.APIKey)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("predict request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read predict response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("predict error (%d): %s", resp.StatusCode, string(data))
	}

	return data, nil
}
