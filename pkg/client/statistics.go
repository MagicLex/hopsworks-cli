package client

import (
	"encoding/json"
	"fmt"
	"strings"
)

type FeatureStatistics struct {
	ID                     int      `json:"id,omitempty"`
	FeatureName            string   `json:"featureName"`
	FeatureType            string   `json:"featureType"`
	Count                  *int64   `json:"count,omitempty"`
	Completeness           *float32 `json:"completeness,omitempty"`
	NumNonNullValues       *int64   `json:"numNonNullValues,omitempty"`
	NumNullValues          *int64   `json:"numNullValues,omitempty"`
	ApproxNumDistinctValues *int64  `json:"approxNumDistinctValues,omitempty"`
	Min                    *float64 `json:"min,omitempty"`
	Max                    *float64 `json:"max,omitempty"`
	Sum                    *float64 `json:"sum,omitempty"`
	Mean                   *float64 `json:"mean,omitempty"`
	Stddev                 *float64 `json:"stddev,omitempty"`
	Distinctness           *float32 `json:"distinctness,omitempty"`
	Entropy                *float32 `json:"entropy,omitempty"`
	Uniqueness             *float32 `json:"uniqueness,omitempty"`
	ExactNumDistinctValues *int64   `json:"exactNumDistinctValues,omitempty"`
}

type Statistics struct {
	ComputationTime             *int64              `json:"computationTime,omitempty"`
	RowPercentage               *float32            `json:"rowPercentage,omitempty"`
	FeatureDescriptiveStatistics []FeatureStatistics `json:"featureDescriptiveStatistics,omitempty"`
	WindowStartCommitTime       *int64              `json:"windowStartCommitTime,omitempty"`
	WindowEndCommitTime         *int64              `json:"windowEndCommitTime,omitempty"`
}

type StatisticsResponse struct {
	Items []Statistics `json:"items"`
	Count int          `json:"count"`
}

type ComputeJobResponse struct {
	ID   int    `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	Type string `json:"type,omitempty"`
}

func (c *Client) GetFeatureGroupStatistics(fgID int, featureNames []string) (*Statistics, error) {
	path := fmt.Sprintf("%s/featuregroups/%d/statistics?fields=content&sort_by=computation_time:desc&offset=0&limit=1",
		c.FSPath(), fgID)

	if len(featureNames) > 0 {
		path += "&feature_names=" + strings.Join(featureNames, ",")
	}

	data, err := c.Get(path)
	if err != nil {
		return nil, err
	}

	var resp StatisticsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse statistics: %w", err)
	}

	if resp.Count == 0 || len(resp.Items) == 0 {
		return nil, nil
	}

	return &resp.Items[0], nil
}

func (c *Client) ComputeFeatureGroupStatistics(fgID int) (*ComputeJobResponse, error) {
	path := fmt.Sprintf("%s/featuregroups/%d/statistics/compute", c.FSPath(), fgID)

	data, err := c.Post(path, nil)
	if err != nil {
		return nil, err
	}

	var job ComputeJobResponse
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, fmt.Errorf("parse compute response: %w", err)
	}
	return &job, nil
}
