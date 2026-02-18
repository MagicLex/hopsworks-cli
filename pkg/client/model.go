package client

import (
	"encoding/json"
	"fmt"
)

type Model struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Version     int                `json:"version"`
	Description string             `json:"description,omitempty"`
	Created     int64              `json:"created,omitempty"`
	Framework   string             `json:"framework,omitempty"`
	Metrics     map[string]float64 `json:"metrics,omitempty"`
	ProjectName string             `json:"projectName,omitempty"`
	UserFullName string            `json:"userFullName,omitempty"`
}

type ModelList struct {
	Items []Model `json:"items"`
	Count int     `json:"count"`
}

// MRPath returns the model registry base path.
// Hopsworks uses the project ID as the model registry ID.
func (c *Client) MRPath() string {
	return fmt.Sprintf("%s/modelregistries/%d/models", c.ProjectPath(), c.Config.ProjectID)
}

func (c *Client) ListModels() ([]Model, error) {
	data, err := c.Get(c.MRPath())
	if err != nil {
		return nil, err
	}

	var list ModelList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("parse models: %w", err)
	}
	if list.Items == nil {
		return []Model{}, nil
	}
	return list.Items, nil
}

func (c *Client) GetModel(name string, version int) (*Model, error) {
	// The API uses name_version as the model identifier
	id := name
	if version > 0 {
		id = fmt.Sprintf("%s_%d", name, version)
	}

	data, err := c.Get(fmt.Sprintf("%s/%s", c.MRPath(), id))
	if err != nil {
		// If no version specified, try listing and finding latest
		if version == 0 {
			return c.getLatestModel(name)
		}
		return nil, err
	}

	var model Model
	if err := json.Unmarshal(data, &model); err != nil {
		return nil, fmt.Errorf("parse model: %w", err)
	}
	return &model, nil
}

func (c *Client) getLatestModel(name string) (*Model, error) {
	models, err := c.ListModels()
	if err != nil {
		return nil, err
	}

	var latest *Model
	for i := range models {
		if models[i].Name == name {
			if latest == nil || models[i].Version > latest.Version {
				latest = &models[i]
			}
		}
	}
	if latest == nil {
		return nil, fmt.Errorf("model %q not found", name)
	}
	return latest, nil
}

func (c *Client) DeleteModel(modelID string) error {
	_, err := c.Delete(fmt.Sprintf("%s/%s", c.MRPath(), modelID))
	return err
}
