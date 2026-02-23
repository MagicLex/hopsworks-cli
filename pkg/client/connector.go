package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
)

// OptionDTO matches the Hopsworks OptionDTO (name/value pair).
type OptionDTO struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// StorageConnector is the superset DTO for all connector types.
// The "type" field is the Jackson discriminator used by Hopsworks REST API.
type StorageConnector struct {
	Type                 string      `json:"type,omitempty"` // e.g. "featurestoreSnowflakeConnectorDTO"
	ID                   int         `json:"id,omitempty"`
	Name                 string      `json:"name"`
	Description          string      `json:"description,omitempty"`
	FeatureStoreID       int         `json:"featurestoreId,omitempty"`
	StorageConnectorType string      `json:"storageConnectorType"` // SNOWFLAKE, JDBC, S3, etc.
	// Snowflake
	URL       string `json:"url,omitempty"`
	User      string `json:"user,omitempty"`
	Password  string `json:"password,omitempty"`
	Token     string `json:"token,omitempty"`
	Database  string `json:"database,omitempty"`
	Schema    string `json:"schema,omitempty"`
	Warehouse string `json:"warehouse,omitempty"`
	Role      string `json:"role,omitempty"`
	Table     string `json:"table,omitempty"`
	// JDBC
	ConnectionString string      `json:"connectionString,omitempty"`
	Arguments        []OptionDTO `json:"arguments,omitempty"`
	// S3
	AccessKey    string `json:"accessKey,omitempty"`
	SecretKey    string `json:"secretKey,omitempty"`
	Bucket       string `json:"bucket,omitempty"`
	Path         string `json:"path,omitempty"`
	Region       string `json:"region,omitempty"`
	IamRole      string `json:"iamRole,omitempty"`
	SessionToken string `json:"sessionToken,omitempty"`
	// BigQuery
	KeyPath                string `json:"keyPath,omitempty"`
	ParentProject          string `json:"parentProject,omitempty"`
	Dataset                string `json:"dataset,omitempty"`
	QueryProject           string `json:"queryProject,omitempty"`
	QueryTable             string `json:"queryTable,omitempty"`
	MaterializationDataset string `json:"materializationDataset,omitempty"`
}

// DataSource represents a browseable data source location (database/schema/table).
type DataSource struct {
	Query    string `json:"query,omitempty"`
	Database string `json:"database,omitempty"`
	Group    string `json:"group,omitempty"` // = schema in Snowflake
	Table    string `json:"table,omitempty"`
	Path     string `json:"path,omitempty"`
}

// DataSourceData holds preview data returned from the data_source/data endpoint.
// The preview field has nested structure: {preview: [{values: [{value0: col, value1: val}, ...]}, ...]}
type DataSourceData struct {
	Limit    int       `json:"limit,omitempty"`
	Features []Feature `json:"features,omitempty"`
	Preview  struct {
		Preview []struct {
			Values []struct {
				Value0 string `json:"value0"`
				Value1 string `json:"value1"`
			} `json:"values"`
		} `json:"preview"`
	} `json:"preview"`
}

// PreviewRows converts the nested preview format into flat column→value maps.
func (d *DataSourceData) PreviewRows() []map[string]string {
	var rows []map[string]string
	for _, row := range d.Preview.Preview {
		r := make(map[string]string)
		for _, v := range row.Values {
			r[v.Value0] = v.Value1
		}
		rows = append(rows, r)
	}
	return rows
}

func (c *Client) connectorsPath() string {
	return fmt.Sprintf("%s/storageconnectors", c.FSPath())
}

func (c *Client) ListStorageConnectors() ([]StorageConnector, error) {
	data, err := c.Get(c.connectorsPath())
	if err != nil {
		return nil, err
	}

	// Try wrapped format first: {items: [...]}
	var list struct {
		Items []StorageConnector `json:"items"`
	}
	if err := json.Unmarshal(data, &list); err == nil {
		if list.Items != nil {
			return list.Items, nil
		}
		return []StorageConnector{}, nil
	}

	// Try direct array
	var connectors []StorageConnector
	if err := json.Unmarshal(data, &connectors); err != nil {
		return nil, fmt.Errorf("parse storage connectors: %w", err)
	}
	return connectors, nil
}

func (c *Client) GetStorageConnector(name string) (*StorageConnector, error) {
	data, err := c.Get(fmt.Sprintf("%s/%s", c.connectorsPath(), name))
	if err != nil {
		return nil, err
	}

	var sc StorageConnector
	if err := json.Unmarshal(data, &sc); err != nil {
		return nil, fmt.Errorf("parse storage connector: %w", err)
	}
	return &sc, nil
}

func (c *Client) CreateStorageConnector(sc *StorageConnector) (*StorageConnector, error) {
	sc.FeatureStoreID = c.Config.FeatureStoreID

	body, err := json.Marshal(sc)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	data, err := c.Post(c.connectorsPath(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	var created StorageConnector
	if err := json.Unmarshal(data, &created); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &created, nil
}

func (c *Client) DeleteStorageConnector(name string) error {
	_, err := c.Delete(fmt.Sprintf("%s/%s", c.connectorsPath(), name))
	return err
}

func (c *Client) GetConnectorDatabases(name string) ([]string, error) {
	path := fmt.Sprintf("%s/%s/data_source/databases", c.connectorsPath(), name)
	data, err := c.Get(path)
	if err != nil {
		return nil, err
	}

	// Response could be a plain array of strings or wrapped
	var databases []string
	if err := json.Unmarshal(data, &databases); err == nil {
		return databases, nil
	}

	// Try wrapped {items: [...]}
	var wrapped struct {
		Items []string `json:"items"`
	}
	if err := json.Unmarshal(data, &wrapped); err == nil && wrapped.Items != nil {
		return wrapped.Items, nil
	}

	// Try as array of DataSource objects
	var sources []DataSource
	if err := json.Unmarshal(data, &sources); err == nil {
		var dbs []string
		for _, s := range sources {
			if s.Database != "" {
				dbs = append(dbs, s.Database)
			}
		}
		return dbs, nil
	}

	return nil, fmt.Errorf("parse databases response: unexpected format")
}

func (c *Client) GetConnectorTables(name, database string) ([]DataSource, error) {
	params := url.Values{}
	if database != "" {
		params.Set("database", database)
	}
	path := fmt.Sprintf("%s/%s/data_source/tables?%s", c.connectorsPath(), name, params.Encode())
	data, err := c.Get(path)
	if err != nil {
		return nil, err
	}

	// Try direct array
	var tables []DataSource
	if err := json.Unmarshal(data, &tables); err == nil {
		return tables, nil
	}

	// Try wrapped
	var wrapped struct {
		Items []DataSource `json:"items"`
	}
	if err := json.Unmarshal(data, &wrapped); err == nil && wrapped.Items != nil {
		return wrapped.Items, nil
	}

	return nil, fmt.Errorf("parse tables response: unexpected format")
}

func (c *Client) GetConnectorData(name string, ds *DataSource) (*DataSourceData, error) {
	params := url.Values{}
	if ds.Database != "" {
		params.Set("database", ds.Database)
	}
	if ds.Table != "" {
		params.Set("table", ds.Table)
	}
	if ds.Group != "" {
		params.Set("group", ds.Group)
	}
	path := fmt.Sprintf("%s/%s/data_source/data?%s", c.connectorsPath(), name, params.Encode())
	data, err := c.Get(path)
	if err != nil {
		return nil, err
	}

	var result DataSourceData
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse data response: %w", err)
	}
	return &result, nil
}

func (c *Client) GetConnectorMetadata(name string, ds *DataSource) (*DataSourceData, error) {
	params := url.Values{}
	if ds.Database != "" {
		params.Set("database", ds.Database)
	}
	if ds.Table != "" {
		params.Set("table", ds.Table)
	}
	if ds.Group != "" {
		params.Set("group", ds.Group)
	}
	path := fmt.Sprintf("%s/%s/data_source/metadata?%s", c.connectorsPath(), name, params.Encode())
	data, err := c.Get(path)
	if err != nil {
		return nil, err
	}

	var result DataSourceData
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse metadata response: %w", err)
	}
	return &result, nil
}
