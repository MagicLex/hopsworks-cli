package client

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// JobRef is a minimal job reference embedded in chart responses.
type JobRef struct {
	ID   int    `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// Chart represents a Hopsworks chart.
// Layout fields (Width, Height, X, Y) are NOT omitempty because the
// dashboard_chart join table has NOT NULL constraints on them.
type Chart struct {
	ID          int     `json:"id,omitempty"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	URL         string  `json:"url"`
	Job         *JobRef `json:"job,omitempty"`
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	X           int     `json:"x"`
	Y           int     `json:"y"`
}

// Dashboard represents a Hopsworks dashboard.
type Dashboard struct {
	ID     int     `json:"id,omitempty"`
	Name   string  `json:"name"`
	Charts []Chart `json:"charts"`
}

func (c *Client) chartsPath() string {
	return fmt.Sprintf("%s/charts", c.ProjectPath())
}

func (c *Client) dashboardsPath() string {
	return fmt.Sprintf("%s/dashboards", c.ProjectPath())
}

// --- Charts ---

func (c *Client) ListCharts() ([]Chart, error) {
	data, err := c.Get(c.chartsPath())
	if err != nil {
		return nil, err
	}

	var list struct {
		Items []Chart `json:"items"`
	}
	if err := json.Unmarshal(data, &list); err == nil && list.Items != nil {
		return list.Items, nil
	}

	var charts []Chart
	if err := json.Unmarshal(data, &charts); err != nil {
		return nil, fmt.Errorf("parse charts: %w", err)
	}
	return charts, nil
}

func (c *Client) GetChart(id int) (*Chart, error) {
	data, err := c.Get(fmt.Sprintf("%s/%d", c.chartsPath(), id))
	if err != nil {
		return nil, err
	}

	var ch Chart
	if err := json.Unmarshal(data, &ch); err != nil {
		return nil, fmt.Errorf("parse chart: %w", err)
	}
	return &ch, nil
}

func (c *Client) CreateChart(ch *Chart) (*Chart, error) {
	body, err := json.Marshal(ch)
	if err != nil {
		return nil, fmt.Errorf("marshal chart: %w", err)
	}

	data, err := c.Post(c.chartsPath(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	var created Chart
	if err := json.Unmarshal(data, &created); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &created, nil
}

func (c *Client) UpdateChart(id int, ch *Chart) (*Chart, error) {
	body, err := json.Marshal(ch)
	if err != nil {
		return nil, fmt.Errorf("marshal chart: %w", err)
	}

	data, err := c.Put(fmt.Sprintf("%s/%d", c.chartsPath(), id), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	var updated Chart
	if err := json.Unmarshal(data, &updated); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &updated, nil
}

func (c *Client) DeleteChart(id int) error {
	_, err := c.Delete(fmt.Sprintf("%s/%d", c.chartsPath(), id))
	return err
}

// --- Dashboards ---

func (c *Client) ListDashboards() ([]Dashboard, error) {
	data, err := c.Get(c.dashboardsPath())
	if err != nil {
		return nil, err
	}

	var list struct {
		Items []Dashboard `json:"items"`
	}
	if err := json.Unmarshal(data, &list); err == nil && list.Items != nil {
		return list.Items, nil
	}

	var dashboards []Dashboard
	if err := json.Unmarshal(data, &dashboards); err != nil {
		return nil, fmt.Errorf("parse dashboards: %w", err)
	}
	return dashboards, nil
}

func (c *Client) GetDashboard(id int) (*Dashboard, error) {
	data, err := c.Get(fmt.Sprintf("%s/%d", c.dashboardsPath(), id))
	if err != nil {
		return nil, err
	}

	var d Dashboard
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("parse dashboard: %w", err)
	}
	return &d, nil
}

func (c *Client) CreateDashboard(d *Dashboard) (*Dashboard, error) {
	if d.Charts == nil {
		d.Charts = []Chart{}
	}
	body, err := json.Marshal(d)
	if err != nil {
		return nil, fmt.Errorf("marshal dashboard: %w", err)
	}

	data, err := c.Post(c.dashboardsPath(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	var created Dashboard
	if err := json.Unmarshal(data, &created); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &created, nil
}

func (c *Client) UpdateDashboard(id int, d *Dashboard) (*Dashboard, error) {
	if d.Charts == nil {
		d.Charts = []Chart{}
	}
	body, err := json.Marshal(d)
	if err != nil {
		return nil, fmt.Errorf("marshal dashboard: %w", err)
	}

	data, err := c.Put(fmt.Sprintf("%s/%d", c.dashboardsPath(), id), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	var updated Dashboard
	if err := json.Unmarshal(data, &updated); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &updated, nil
}

func (c *Client) DeleteDashboard(id int) error {
	_, err := c.Delete(fmt.Sprintf("%s/%d", c.dashboardsPath(), id))
	return err
}
