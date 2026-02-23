package client

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type TrainingDataset struct {
	ID          int    `json:"id"`
	Version     int    `json:"version"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	DataFormat  string `json:"dataFormat,omitempty"`
	Created     string `json:"created,omitempty"`
	Location    string `json:"location,omitempty"`
}

type TrainingDatasetList struct {
	Items []TrainingDataset `json:"items"`
	Count int               `json:"count"`
}

func (c *Client) ListTrainingDatasets(fvName string, fvVersion int) ([]TrainingDataset, error) {
	path := fmt.Sprintf("%s/featureview/%s/version/%d/trainingdatasets", c.FSPath(), fvName, fvVersion)
	data, err := c.Get(path)
	if err != nil {
		return nil, err
	}

	var tdList TrainingDatasetList
	if err := json.Unmarshal(data, &tdList); err == nil {
		if tdList.Items != nil {
			return tdList.Items, nil
		}
		return []TrainingDataset{}, nil
	}

	var tds []TrainingDataset
	if err := json.Unmarshal(data, &tds); err != nil {
		return nil, fmt.Errorf("parse training datasets: %w", err)
	}
	return tds, nil
}

func (c *Client) CreateTrainingDataset(fvName string, fvVersion int, description string, dataFormat string) (*TrainingDataset, error) {
	req := map[string]interface{}{
		"type":                "trainingDatasetDTO",
		"trainingDatasetType": "HOPSFS_TRAINING_DATASET",
	}
	if description != "" {
		req["description"] = description
	}
	if dataFormat != "" {
		req["dataFormat"] = dataFormat
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	path := fmt.Sprintf("%s/featureview/%s/version/%d/trainingdatasets", c.FSPath(), fvName, fvVersion)
	data, err := c.Post(path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	var td TrainingDataset
	if err := json.Unmarshal(data, &td); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &td, nil
}

func (c *Client) DeleteTrainingDataset(fvName string, fvVersion int, tdVersion int) error {
	path := fmt.Sprintf("%s/featureview/%s/version/%d/trainingdatasets/version/%d", c.FSPath(), fvName, fvVersion, tdVersion)
	_, err := c.Delete(path)
	return err
}
