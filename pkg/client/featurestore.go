package client

import (
	"encoding/json"
	"fmt"
)

type FeatureStore struct {
	FeaturestoreID   int    `json:"featurestoreId"`
	FeaturestoreName string `json:"featurestoreName"`
	ProjectID        int    `json:"projectId"`
}

func (c *Client) ListFeatureStores() ([]FeatureStore, error) {
	data, err := c.Get(fmt.Sprintf("/hopsworks-api/api/project/%d/featurestores", c.Config.ProjectID))
	if err != nil {
		return nil, err
	}

	var stores []FeatureStore
	if err := json.Unmarshal(data, &stores); err != nil {
		return nil, fmt.Errorf("parse feature stores: %w", err)
	}
	return stores, nil
}
