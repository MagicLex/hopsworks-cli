package client

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type FeatureView struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Version     int       `json:"version"`
	Description string    `json:"description,omitempty"`
	Created     string    `json:"created,omitempty"`
	Features    []Feature `json:"features,omitempty"`
	Labels      []string  `json:"label,omitempty"`
}

type FeatureViewList struct {
	Items []FeatureView `json:"items"`
	Count int           `json:"count"`
}

func (c *Client) ListFeatureViews() ([]FeatureView, error) {
	data, err := c.Get(fmt.Sprintf("%s/featureview", c.FSPath()))
	if err != nil {
		return nil, err
	}

	var fvList FeatureViewList
	if err := json.Unmarshal(data, &fvList); err == nil {
		if fvList.Items != nil {
			return fvList.Items, nil
		}
		// count: 0 with no items means empty
		if fvList.Count == 0 {
			return []FeatureView{}, nil
		}
	}

	var fvs []FeatureView
	if err := json.Unmarshal(data, &fvs); err != nil {
		return nil, fmt.Errorf("parse feature views: %w", err)
	}
	return fvs, nil
}

func (c *Client) GetFeatureView(name string, version int) (*FeatureView, error) {
	var path string
	if version > 0 {
		path = fmt.Sprintf("%s/featureview/%s/version/%d", c.FSPath(), name, version)
	} else {
		path = fmt.Sprintf("%s/featureview/%s", c.FSPath(), name)
	}

	data, err := c.Get(path)
	if err != nil {
		return nil, err
	}

	// May return list or single
	var fvList FeatureViewList
	if err := json.Unmarshal(data, &fvList); err == nil && fvList.Items != nil && len(fvList.Items) > 0 {
		// Return latest version
		latest := fvList.Items[0]
		for _, fv := range fvList.Items {
			if fv.Version > latest.Version {
				latest = fv
			}
		}
		return &latest, nil
	}

	var fv FeatureView
	if err := json.Unmarshal(data, &fv); err != nil {
		return nil, fmt.Errorf("parse feature view: %w", err)
	}
	return &fv, nil
}

type CreateFeatureViewRequest struct {
	Name             string `json:"name"`
	Version          int    `json:"version"`
	Description      string `json:"description,omitempty"`
	FeatureStoreName string `json:"-"`
	Query            struct {
		LeftFeatureGroup struct {
			ID int `json:"id"`
		} `json:"leftFeatureGroup"`
		LeftFeatures []struct {
			Name string `json:"name"`
		} `json:"leftFeatures"`
	} `json:"query"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"label,omitempty"`
}

func (c *Client) CreateFeatureView(name string, version int, description string, fgID int, features []string, labels []string) (*FeatureView, error) {
	req := map[string]interface{}{
		"name":    name,
		"version": version,
	}
	if description != "" {
		req["description"] = description
	}

	query := map[string]interface{}{
		"leftFeatureGroup": map[string]int{"id": fgID},
	}
	if len(features) > 0 {
		var leftFeatures []map[string]string
		for _, f := range features {
			leftFeatures = append(leftFeatures, map[string]string{"name": f})
		}
		query["leftFeatures"] = leftFeatures
	}
	req["query"] = query

	if len(labels) > 0 {
		var labelList []map[string]string
		for _, l := range labels {
			labelList = append(labelList, map[string]string{"name": l})
		}
		req["label"] = labelList
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	data, err := c.Post(fmt.Sprintf("%s/featureview", c.FSPath()), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	var fv FeatureView
	if err := json.Unmarshal(data, &fv); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &fv, nil
}

func (c *Client) DeleteFeatureView(name string, version int) error {
	var path string
	if version > 0 {
		path = fmt.Sprintf("%s/featureview/%s/version/%d", c.FSPath(), name, version)
	} else {
		path = fmt.Sprintf("%s/featureview/%s", c.FSPath(), name)
	}
	_, err := c.Delete(path)
	return err
}
