package client

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type TransformationFunction struct {
	ID             int          `json:"id,omitempty"`
	Version        int          `json:"version"`
	FeatureStoreID int          `json:"featurestoreId"`
	HopsworksUdf   HopsworksUdf `json:"hopsworksUdf"`
}

type HopsworksUdf struct {
	SourceCode                          string   `json:"sourceCode"`
	Name                                string   `json:"name"`
	OutputTypes                         []string `json:"outputTypes"`
	TransformationFeatures              []string `json:"transformationFeatures"`
	TransformationFunctionArgumentNames []string `json:"transformationFunctionArgumentNames"`
	DroppedArgumentNames                []string `json:"droppedArgumentNames"`
	StatisticsArgumentNames             []string `json:"statisticsArgumentNames"`
	ExecutionMode                       string   `json:"executionMode"`
}

type TransformationFunctionList struct {
	Items []TransformationFunction `json:"items"`
	Count int                      `json:"count"`
}

func (c *Client) ListTransformationFunctions() ([]TransformationFunction, error) {
	data, err := c.Get(fmt.Sprintf("%s/transformationfunctions", c.FSPath()))
	if err != nil {
		return nil, err
	}

	var tfList TransformationFunctionList
	if err := json.Unmarshal(data, &tfList); err == nil {
		if tfList.Items != nil {
			return tfList.Items, nil
		}
		return []TransformationFunction{}, nil
	}

	var tfs []TransformationFunction
	if err := json.Unmarshal(data, &tfs); err != nil {
		return nil, fmt.Errorf("parse transformation functions: %w", err)
	}
	return tfs, nil
}

func (c *Client) GetTransformationFunction(name string, version int) (*TransformationFunction, error) {
	path := fmt.Sprintf("%s/transformationfunctions?name=%s", c.FSPath(), name)
	if version > 0 {
		path += fmt.Sprintf("&version=%d", version)
	}

	data, err := c.Get(path)
	if err != nil {
		return nil, err
	}

	var tfList TransformationFunctionList
	if err := json.Unmarshal(data, &tfList); err == nil && tfList.Items != nil && len(tfList.Items) > 0 {
		return &tfList.Items[0], nil
	}

	var tf TransformationFunction
	if err := json.Unmarshal(data, &tf); err != nil {
		return nil, fmt.Errorf("parse transformation function: %w", err)
	}
	if tf.ID == 0 {
		return nil, fmt.Errorf("transformation function '%s' not found", name)
	}
	return &tf, nil
}

func (c *Client) CreateTransformationFunction(tf *TransformationFunction) (*TransformationFunction, error) {
	tf.FeatureStoreID = c.Config.FeatureStoreID

	body, err := json.Marshal(tf)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	data, err := c.Post(fmt.Sprintf("%s/transformationfunctions", c.FSPath()), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	var result TransformationFunction
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &result, nil
}
