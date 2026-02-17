package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

type Feature struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Primary     bool   `json:"primary"`
	Partition   bool   `json:"partition,omitempty"`
}

type FeatureGroup struct {
	ID            int       `json:"id"`
	Name          string    `json:"name"`
	Version       int       `json:"version"`
	Description   string    `json:"description,omitempty"`
	Created       string    `json:"created,omitempty"`
	OnlineEnabled bool      `json:"onlineEnabled"`
	TimeTravelFormat string `json:"timeTravelFormat,omitempty"`
	Features      []Feature `json:"features,omitempty"`
	EventTime     string    `json:"eventTime,omitempty"`
	NumRows       *int64    `json:"numRows,omitempty"`
	Location      string    `json:"location,omitempty"`
}

type FeatureGroupList struct {
	Items []FeatureGroup `json:"items"`
	Count int            `json:"count"`
}

func (c *Client) ListFeatureGroups() ([]FeatureGroup, error) {
	data, err := c.Get(fmt.Sprintf("%s/featuregroups", c.FSPath()))
	if err != nil {
		return nil, err
	}

	// The API might return a list or a wrapped object
	var fgList FeatureGroupList
	if err := json.Unmarshal(data, &fgList); err == nil && fgList.Items != nil {
		return fgList.Items, nil
	}

	// Try as direct array
	var fgs []FeatureGroup
	if err := json.Unmarshal(data, &fgs); err != nil {
		return nil, fmt.Errorf("parse feature groups: %w", err)
	}
	return fgs, nil
}

func (c *Client) GetFeatureGroup(name string, version int) (*FeatureGroup, error) {
	path := fmt.Sprintf("%s/featuregroups/%s", c.FSPath(), name)
	if version > 0 {
		path += fmt.Sprintf("?version=%d", version)
	}

	data, err := c.Get(path)
	if err != nil {
		return nil, err
	}

	// Try as plain array first (by-name endpoint returns [fg1, fg2, ...])
	var fgs []FeatureGroup
	if err := json.Unmarshal(data, &fgs); err == nil && len(fgs) > 0 {
		if version > 0 {
			for _, fg := range fgs {
				if fg.Version == version {
					return &fg, nil
				}
			}
		}
		// Return latest version
		latest := fgs[0]
		for _, fg := range fgs {
			if fg.Version > latest.Version {
				latest = fg
			}
		}
		return &latest, nil
	}

	// Try wrapped format {items: [...]}
	var fgList FeatureGroupList
	if err := json.Unmarshal(data, &fgList); err == nil && fgList.Items != nil && len(fgList.Items) > 0 {
		if version > 0 {
			for _, item := range fgList.Items {
				if item.Version == version {
					return &item, nil
				}
			}
		}
		return &fgList.Items[0], nil
	}

	// Try single object
	var fg FeatureGroup
	if err := json.Unmarshal(data, &fg); err != nil {
		return nil, fmt.Errorf("parse feature group: %w", err)
	}
	return &fg, nil
}

func (c *Client) PreviewFeatureGroup(fgID int, n int) ([]map[string]interface{}, error) {
	path := fmt.Sprintf("%s/featuregroups/%d/preview?storage=offline&limit=%d", c.FSPath(), fgID, n)

	data, err := c.Get(path)
	if err != nil {
		return nil, err
	}

	// The preview endpoint returns items with rows
	var result struct {
		Items []struct {
			Type string `json:"type"`
			Rows []struct {
				Values []struct {
					ColumnName string      `json:"columnName"`
					Value      interface{} `json:"columnValue"`
				} `json:"values"`
			} `json:"rows"`
		} `json:"items"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse preview: %w", err)
	}

	var rows []map[string]interface{}
	for _, item := range result.Items {
		for _, row := range item.Rows {
			r := make(map[string]interface{})
			for _, v := range row.Values {
				r[v.ColumnName] = v.Value
			}
			rows = append(rows, r)
		}
	}
	return rows, nil
}

type CreateFeatureGroupRequest struct {
	Name             string    `json:"name"`
	Version          int       `json:"version"`
	Description      string    `json:"description,omitempty"`
	OnlineEnabled    bool      `json:"onlineEnabled"`
	EventTime        string    `json:"eventTime,omitempty"`
	Features         []Feature `json:"features"`
	TimeTravelFormat string    `json:"timeTravelFormat,omitempty"`
	Type             string    `json:"type"`
	FeatureStoreID   int       `json:"featurestoreId"`
}

func (c *Client) CreateFeatureGroup(req *CreateFeatureGroupRequest) (*FeatureGroup, error) {
	// Set required fields from client config
	if req.Type == "" {
		req.Type = "cachedFeaturegroupDTO"
	}
	req.FeatureStoreID = c.Config.FeatureStoreID

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	path := fmt.Sprintf("%s/featuregroups", c.FSPath())
	data, err := c.Post(path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	var fg FeatureGroup
	if err := json.Unmarshal(data, &fg); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &fg, nil
}

func (c *Client) DeleteFeatureGroup(fgID int) error {
	_, err := c.Delete(fmt.Sprintf("%s/featuregroups/%d", c.FSPath(), fgID))
	return err
}

// ParsePrimaryKeys marks features as primary in a feature list
func ParseFeatures(featureNames []string, types []string, primaryKeys []string) []Feature {
	pkSet := make(map[string]bool)
	for _, pk := range primaryKeys {
		pkSet[strings.TrimSpace(pk)] = true
	}

	var features []Feature
	for i, name := range featureNames {
		t := "string"
		if i < len(types) {
			t = types[i]
		}
		features = append(features, Feature{
			Name:    strings.TrimSpace(name),
			Type:    t,
			Primary: pkSet[strings.TrimSpace(name)],
		})
	}
	return features
}
