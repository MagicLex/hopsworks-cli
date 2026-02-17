package client

import (
	"encoding/json"
	"fmt"
)

type Job struct {
	Name         string `json:"name"`
	ID           int    `json:"id"`
	CreationTime string `json:"creationTime,omitempty"`
	JobType      string `json:"jobType,omitempty"`
	Creator      *struct {
		Email string `json:"email,omitempty"`
	} `json:"creator,omitempty"`
	Config *struct {
		Type string `json:"type,omitempty"`
	} `json:"config,omitempty"`
}

type JobList struct {
	Items []Job `json:"items"`
	Count int   `json:"count"`
}

func (c *Client) ListJobs() ([]Job, error) {
	data, err := c.Get(fmt.Sprintf("%s/jobs", c.ProjectPath()))
	if err != nil {
		return nil, err
	}

	var jobList JobList
	if err := json.Unmarshal(data, &jobList); err == nil {
		if jobList.Items != nil {
			return jobList.Items, nil
		}
		if jobList.Count == 0 {
			return []Job{}, nil
		}
	}

	var jobs []Job
	if err := json.Unmarshal(data, &jobs); err != nil {
		return nil, fmt.Errorf("parse jobs: %w", err)
	}
	return jobs, nil
}
