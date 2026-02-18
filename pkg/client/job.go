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

type Execution struct {
	ID             int     `json:"id"`
	State          string  `json:"state"`
	FinalStatus    string  `json:"finalStatus"`
	SubmissionTime string  `json:"submissionTime"`
	Duration       int64   `json:"duration"`
	Progress       float64 `json:"progress"`
	AppID          string  `json:"appId"`
	HDFSUser       string  `json:"hdfsUser"`
}

type ExecutionList struct {
	Items []Execution `json:"items"`
	Count int         `json:"count"`
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

// GetLatestExecution returns the most recent execution for a job.
func (c *Client) GetLatestExecution(jobName string) (*Execution, error) {
	path := fmt.Sprintf("%s/jobs/%s/executions?sort_by=submissionTime:desc&limit=1", c.ProjectPath(), jobName)
	data, err := c.Get(path)
	if err != nil {
		return nil, err
	}

	var list ExecutionList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("parse executions: %w", err)
	}
	if len(list.Items) == 0 {
		return nil, nil
	}
	return &list.Items[0], nil
}

// IsExecutionTerminal returns true if the execution state is a final state.
func IsExecutionTerminal(state string) bool {
	switch state {
	case "FINISHED", "FAILED", "KILLED", "FRAMEWORK_FAILURE", "APP_MASTER_START_FAILED", "INITIALIZATION_FAILED":
		return true
	}
	return false
}
