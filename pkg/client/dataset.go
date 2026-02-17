package client

import (
	"encoding/json"
	"fmt"
	"strings"
)

type DatasetFile struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	DatasetType string `json:"datasetType,omitempty"`
	Dir         bool   `json:"dir"`
	Size        int64  `json:"size"`
}

type datasetAPIItem struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	DatasetType string `json:"datasetType"`
}

type datasetAPIList struct {
	Items []datasetAPIItem `json:"items"`
	Count int              `json:"count"`
}

func (c *Client) ListDatasets(path string) ([]DatasetFile, error) {
	path = strings.TrimPrefix(path, "/")

	apiPath := fmt.Sprintf("%s/dataset/%s?action=listing&offset=0&limit=100", c.ProjectPath(), path)
	data, err := c.Get(apiPath)
	if err != nil {
		return nil, err
	}

	var dsList datasetAPIList
	if err := json.Unmarshal(data, &dsList); err != nil {
		return nil, fmt.Errorf("parse datasets: %w", err)
	}

	var files []DatasetFile
	for _, item := range dsList.Items {
		files = append(files, DatasetFile{
			Name:        item.Name,
			Description: item.Description,
			DatasetType: item.DatasetType,
			Dir:         item.DatasetType == "DATASET",
		})
	}
	return files, nil
}

func (c *Client) MkDir(path string) error {
	path = strings.TrimPrefix(path, "/")
	apiPath := fmt.Sprintf("%s/dataset/%s?action=create&type=DATASET", c.ProjectPath(), path)
	_, err := c.Post(apiPath, nil)
	return err
}
