package client

import (
	"encoding/json"
	"fmt"
)

type Project struct {
	ProjectID   int    `json:"projectId"`
	ProjectName string `json:"projectName"`
	Description string `json:"description,omitempty"`
	Created     string `json:"created,omitempty"`
}

// projectWrapper handles the nested {"project": {...}} response format
type projectWrapper struct {
	Project struct {
		ID          int    `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Created     string `json:"created"`
	} `json:"project"`
}

func (c *Client) ListProjects() ([]Project, error) {
	data, err := c.Get("/hopsworks-api/api/project")
	if err != nil {
		return nil, err
	}

	// Try nested format first: [{"project": {"id": ..., "name": ...}}]
	var wrappers []projectWrapper
	if err := json.Unmarshal(data, &wrappers); err == nil && len(wrappers) > 0 && wrappers[0].Project.ID > 0 {
		var projects []Project
		for _, w := range wrappers {
			projects = append(projects, Project{
				ProjectID:   w.Project.ID,
				ProjectName: w.Project.Name,
				Description: w.Project.Description,
				Created:     w.Project.Created,
			})
		}
		return projects, nil
	}

	// Fallback: flat format
	var projects []Project
	if err := json.Unmarshal(data, &projects); err != nil {
		return nil, fmt.Errorf("parse projects: %w", err)
	}
	return projects, nil
}

func (c *Client) GetProject(id int) (*Project, error) {
	data, err := c.Get(fmt.Sprintf("/hopsworks-api/api/project/%d", id))
	if err != nil {
		return nil, err
	}

	// Try nested format
	var w projectWrapper
	if err := json.Unmarshal(data, &w); err == nil && w.Project.ID > 0 {
		return &Project{
			ProjectID:   w.Project.ID,
			ProjectName: w.Project.Name,
			Description: w.Project.Description,
			Created:     w.Project.Created,
		}, nil
	}

	var project Project
	if err := json.Unmarshal(data, &project); err != nil {
		return nil, fmt.Errorf("parse project: %w", err)
	}
	return &project, nil
}

func (c *Client) GetProjectByName(name string) (*Project, error) {
	projects, err := c.ListProjects()
	if err != nil {
		return nil, err
	}
	for _, p := range projects {
		if p.ProjectName == name {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("project '%s' not found", name)
}
