package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type Job struct {
	Name         string `json:"name"`
	ID           int    `json:"id"`
	CreationTime string `json:"creationTime,omitempty"`
	JobType      string `json:"jobType,omitempty"`
	Creator      *struct {
		Email string `json:"email,omitempty"`
	} `json:"creator,omitempty"`
	Config json.RawMessage `json:"config,omitempty"`
}

// ConfigType extracts the "type" field from the job config JSON.
func (j *Job) ConfigType() string {
	if j.Config == nil {
		return ""
	}
	var cfg struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(j.Config, &cfg) == nil {
		return cfg.Type
	}
	return ""
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

type ExecutionLog struct {
	Log  string `json:"log"`
	Type string `json:"type"`
}

type JobSchedule struct {
	ID                    int    `json:"id,omitempty"`
	CronExpression        string `json:"cronExpression"`
	StartDateTime         *int64 `json:"startDateTime"`
	EndDateTime           *int64 `json:"endDateTime,omitempty"`
	Enabled               *bool  `json:"enabled"`
	NextExecutionDateTime *int64 `json:"nextExecutionDateTime,omitempty"`
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

func (c *Client) GetJob(name string) (*Job, error) {
	data, err := c.Get(fmt.Sprintf("%s/jobs/%s", c.ProjectPath(), name))
	if err != nil {
		return nil, err
	}
	var job Job
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, fmt.Errorf("parse job: %w", err)
	}
	return &job, nil
}

// CreateJob creates or updates a job. config is the JobConfiguration JSON body directly.
func (c *Client) CreateJob(name string, config json.RawMessage) (*Job, error) {
	data, err := c.Put(fmt.Sprintf("%s/jobs/%s", c.ProjectPath(), name), bytes.NewReader(config))
	if err != nil {
		return nil, err
	}
	var job Job
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, fmt.Errorf("parse created job: %w", err)
	}
	return &job, nil
}

func (c *Client) DeleteJob(name string) error {
	_, err := c.Delete(fmt.Sprintf("%s/jobs/%s", c.ProjectPath(), name))
	return err
}

func (c *Client) GetSchedule(jobName string) (*JobSchedule, error) {
	data, err := c.Get(fmt.Sprintf("%s/jobs/%s/schedule/v2", c.ProjectPath(), jobName))
	if err != nil {
		return nil, err
	}
	var sched JobSchedule
	if err := json.Unmarshal(data, &sched); err != nil {
		return nil, fmt.Errorf("parse schedule: %w", err)
	}
	return &sched, nil
}

func (c *Client) CreateSchedule(jobName string, sched *JobSchedule) (*JobSchedule, error) {
	payload, _ := json.Marshal(sched)
	data, err := c.Post(fmt.Sprintf("%s/jobs/%s/schedule/v2", c.ProjectPath(), jobName), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	var result JobSchedule
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse schedule: %w", err)
	}
	return &result, nil
}

func (c *Client) UpdateSchedule(jobName string, sched *JobSchedule) (*JobSchedule, error) {
	payload, _ := json.Marshal(sched)
	data, err := c.Put(fmt.Sprintf("%s/jobs/%s/schedule/v2", c.ProjectPath(), jobName), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	var result JobSchedule
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse schedule: %w", err)
	}
	return &result, nil
}

func (c *Client) DeleteSchedule(jobName string) error {
	_, err := c.Delete(fmt.Sprintf("%s/jobs/%s/schedule/v2", c.ProjectPath(), jobName))
	return err
}

// RunJob starts a new execution. args is optional (passed as text/plain body).
func (c *Client) RunJob(name string, args string) (*Execution, error) {
	path := fmt.Sprintf("%s/jobs/%s/executions", c.ProjectPath(), name)

	var body io.Reader
	if args != "" {
		body = strings.NewReader(args)
	}

	data, err := c.postText(path, body)
	if err != nil {
		return nil, err
	}
	var exec Execution
	if err := json.Unmarshal(data, &exec); err != nil {
		return nil, fmt.Errorf("parse execution: %w", err)
	}
	return &exec, nil
}

// StopExecution stops a running execution.
func (c *Client) StopExecution(jobName string, execID int) (*Execution, error) {
	path := fmt.Sprintf("%s/jobs/%s/executions/%d/status", c.ProjectPath(), jobName, execID)
	payload := []byte(`{"state":"stopped"}`)
	data, err := c.Put(path, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	var exec Execution
	if err := json.Unmarshal(data, &exec); err != nil {
		return nil, fmt.Errorf("parse execution: %w", err)
	}
	return &exec, nil
}

func (c *Client) GetExecutions(jobName string, limit int) ([]Execution, error) {
	if limit <= 0 {
		limit = 10
	}
	path := fmt.Sprintf("%s/jobs/%s/executions?sort_by=submissionTime:desc&limit=%d", c.ProjectPath(), jobName, limit)
	data, err := c.Get(path)
	if err != nil {
		return nil, err
	}
	var list ExecutionList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("parse executions: %w", err)
	}
	return list.Items, nil
}

func (c *Client) GetExecutionLogs(jobName string, execID int, logType string) (*ExecutionLog, error) {
	if logType == "" {
		logType = "OUT"
	}
	logType = strings.ToUpper(logType)
	path := fmt.Sprintf("%s/jobs/%s/executions/%d/log/%s", c.ProjectPath(), jobName, execID, logType)
	data, err := c.Get(path)
	if err != nil {
		return nil, err
	}
	var log ExecutionLog
	if err := json.Unmarshal(data, &log); err != nil {
		return nil, fmt.Errorf("parse log: %w", err)
	}
	return &log, nil
}

// GetDefaultConfig fetches the system default config for a job type.
// Tries /jobs/{type}/configuration first (system default), falls back to /jobconfig/{type} (user default).
func (c *Client) GetDefaultConfig(jobType string) (json.RawMessage, error) {
	jt := strings.ToLower(jobType)
	// System default endpoint
	path := fmt.Sprintf("%s/jobs/%s/configuration", c.ProjectPath(), jt)
	data, err := c.Get(path)
	if err == nil {
		return json.RawMessage(data), nil
	}
	// Fallback: user-saved default
	path = fmt.Sprintf("%s/jobconfig/%s", c.ProjectPath(), strings.ToUpper(jobType))
	data, err = c.Get(path)
	if err == nil {
		return json.RawMessage(data), nil
	}
	return nil, fmt.Errorf("no default config for job type %s: %w", jobType, err)
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

// postText sends a POST with text/plain content type (used for job execution args).
func (c *Client) postText(path string, body io.Reader) ([]byte, error) {
	url := c.baseURL() + path
	if body == nil {
		body = strings.NewReader("")
	}

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if c.Config.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.Config.JWTToken)
	} else {
		req.Header.Set("Authorization", "ApiKey "+c.Config.APIKey)
	}
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp struct {
			ErrorMsg string `json:"errorMsg"`
			UsrMsg   string `json:"usrMsg"`
			DevMsg   string `json:"devMsg"`
		}
		if json.Unmarshal(data, &errResp) == nil {
			msg := errResp.UsrMsg
			if msg == "" {
				msg = errResp.ErrorMsg
			}
			if msg != "" && errResp.DevMsg != "" && errResp.DevMsg != msg {
				return nil, fmt.Errorf("API error (%d): %s — %s", resp.StatusCode, msg, errResp.DevMsg)
			}
			if msg != "" {
				return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, msg)
			}
		}
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(data))
	}

	return data, nil
}
