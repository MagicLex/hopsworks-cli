package client

import (
	"encoding/json"
	"fmt"
	"strings"
)

type DatasetFile struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Dir  bool   `json:"dir"`
	Size int64  `json:"size"`
}

type DatasetAttributes struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Dir  bool   `json:"dir"`
	Size int64  `json:"size"`
}

type DatasetItem struct {
	Attributes DatasetAttributes `json:"attributes"`
}

type DatasetList struct {
	Items []DatasetItem `json:"items"`
	Count int           `json:"count"`
}

func (c *Client) ListDatasets(path string) ([]DatasetFile, error) {
	if path == "" {
		path = ""
	}
	// Clean path
	path = strings.TrimPrefix(path, "/")

	apiPath := fmt.Sprintf("%s/dataset/%s?action=listing&offset=0&limit=100", c.ProjectPath(), path)
	data, err := c.Get(apiPath)
	if err != nil {
		return nil, err
	}

	var dsList DatasetList
	if err := json.Unmarshal(data, &dsList); err == nil && dsList.Items != nil {
		var files []DatasetFile
		for _, item := range dsList.Items {
			files = append(files, DatasetFile{
				Name: item.Attributes.Name,
				Path: item.Attributes.Path,
				Dir:  item.Attributes.Dir,
				Size: item.Attributes.Size,
			})
		}
		return files, nil
	}

	var files []DatasetFile
	if err := json.Unmarshal(data, &files); err != nil {
		return nil, fmt.Errorf("parse datasets: %w", err)
	}
	return files, nil
}

func (c *Client) MkDir(path string) error {
	path = strings.TrimPrefix(path, "/")
	apiPath := fmt.Sprintf("%s/dataset/%s?action=create&type=DATASET", c.ProjectPath(), path)
	_, err := c.Post(apiPath, nil)
	return err
}
