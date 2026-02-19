package client

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type FeatureView struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Version     int       `json:"version"`
	Description string    `json:"description,omitempty"`
	Created     string    `json:"created,omitempty"`
	Features    []Feature `json:"features,omitempty"`
	Labels      []string  `json:"label,omitempty"`
}

type FeatureViewList struct {
	Items []FeatureView `json:"items"`
	Count int           `json:"count"`
}

func (c *Client) ListFeatureViews() ([]FeatureView, error) {
	data, err := c.Get(fmt.Sprintf("%s/featureview", c.FSPath()))
	if err != nil {
		return nil, err
	}

	var fvList FeatureViewList
	if err := json.Unmarshal(data, &fvList); err == nil {
		if fvList.Items != nil {
			return fvList.Items, nil
		}
		// count: 0 with no items means empty
		if fvList.Count == 0 {
			return []FeatureView{}, nil
		}
	}

	var fvs []FeatureView
	if err := json.Unmarshal(data, &fvs); err != nil {
		return nil, fmt.Errorf("parse feature views: %w", err)
	}
	return fvs, nil
}

func (c *Client) GetFeatureView(name string, version int) (*FeatureView, error) {
	var path string
	if version > 0 {
		path = fmt.Sprintf("%s/featureview/%s/version/%d", c.FSPath(), name, version)
	} else {
		path = fmt.Sprintf("%s/featureview/%s", c.FSPath(), name)
	}

	data, err := c.Get(path)
	if err != nil {
		return nil, err
	}

	// May return list or single
	var fvList FeatureViewList
	if err := json.Unmarshal(data, &fvList); err == nil && fvList.Items != nil && len(fvList.Items) > 0 {
		// Return latest version
		latest := fvList.Items[0]
		for _, fv := range fvList.Items {
			if fv.Version > latest.Version {
				latest = fv
			}
		}
		return &latest, nil
	}

	var fv FeatureView
	if err := json.Unmarshal(data, &fv); err != nil {
		return nil, fmt.Errorf("parse feature view: %w", err)
	}
	return &fv, nil
}

// FVTransformSpec maps a transformation function to a feature column.
type FVTransformSpec struct {
	TF     *TransformationFunction
	Column string // target feature column name
}

// FVJoinSpec describes a join for feature view creation.
type FVJoinSpec struct {
	FG      *FeatureGroup
	LeftOn  []string // join keys on the left (base) FG
	RightOn []string // join keys on the right (joined) FG
	Type    string   // "LEFT", "INNER", "RIGHT", "FULL"
	Prefix  string   // optional prefix for right FG features
}

func (c *Client) CreateFeatureView(name string, version int, description string, baseFG *FeatureGroup, features []string, labels []string, joins []FVJoinSpec, transforms []FVTransformSpec) (*FeatureView, error) {
	req := map[string]interface{}{
		"name":           name,
		"version":        version,
		"type":           "featureViewDTO",
		"featurestoreId": c.Config.FeatureStoreID,
	}
	if description != "" {
		req["description"] = description
	}

	fsName := c.Config.Project + "_featurestore"

	// Build base query
	query := map[string]interface{}{
		"leftFeatureGroup": c.buildFGRef(baseFG),
		"leftFeatures":     c.buildFeatureList(baseFG, features),
		"featureStoreId":   c.Config.FeatureStoreID,
		"featureStoreName": fsName,
		"hiveEngine":       true,
	}

	// Build joins array, nesting when leftOn belongs to a previously-joined FG
	if len(joins) > 0 {
		var topJoins []map[string]interface{}
		// Track join queries by FG name so we can nest sub-joins
		type joinNode struct {
			query map[string]interface{} // the inner "query" object
			fg    *FeatureGroup
		}
		var nodes []joinNode

		for _, j := range joins {
			// All features from the join FG
			var rightFeatureNames []string
			for _, f := range j.FG.Features {
				rightFeatureNames = append(rightFeatureNames, f.Name)
			}

			joinQuery := map[string]interface{}{
				"leftFeatureGroup": c.buildFGRef(j.FG),
				"leftFeatures":     c.buildFeatureList(j.FG, rightFeatureNames),
				"featureStoreId":   c.Config.FeatureStoreID,
				"featureStoreName": fsName,
				"hiveEngine":       true,
				"joins":            []interface{}{},
			}

			joinEntry := map[string]interface{}{
				"query": joinQuery,
				"type":  j.Type,
			}

			// Join keys: "on" when same name, "leftOn"+"rightOn" when different
			if len(j.LeftOn) > 0 && j.LeftOn[0] == j.RightOn[0] {
				joinEntry["on"] = j.LeftOn
				joinEntry["leftOn"] = []string{}
				joinEntry["rightOn"] = []string{}
			} else {
				joinEntry["on"] = []string{}
				joinEntry["leftOn"] = j.LeftOn
				joinEntry["rightOn"] = j.RightOn
			}

			if j.Prefix != "" {
				joinEntry["prefix"] = j.Prefix
			}

			// Check if leftOn exists in the base FG
			leftOnInBase := false
			if len(j.LeftOn) > 0 {
				for _, f := range baseFG.Features {
					if f.Name == j.LeftOn[0] {
						leftOnInBase = true
						break
					}
				}
			}

			if leftOnInBase {
				topJoins = append(topJoins, joinEntry)
			} else {
				// Find which previously-joined FG owns the leftOn feature and nest there
				nested := false
				for _, prev := range nodes {
					for _, f := range prev.fg.Features {
						if f.Name == j.LeftOn[0] {
							prevJoins := prev.query["joins"].([]interface{})
							prev.query["joins"] = append(prevJoins, joinEntry)
							nested = true
							break
						}
					}
					if nested {
						break
					}
				}
				if !nested {
					// Fallback: add to top level, let server validate
					topJoins = append(topJoins, joinEntry)
				}
			}

			nodes = append(nodes, joinNode{query: joinQuery, fg: j.FG})
		}
		query["joins"] = topJoins
	}

	req["query"] = query

	if len(labels) > 0 {
		var labelList []map[string]string
		for _, l := range labels {
			labelList = append(labelList, map[string]string{"name": l})
		}
		req["label"] = labelList
	}

	if len(transforms) > 0 {
		var tfList []map[string]interface{}
		for _, t := range transforms {
			// Clone the UDF but set transformationFeatures to the target column
			udf := map[string]interface{}{
				"sourceCode":                          t.TF.HopsworksUdf.SourceCode,
				"name":                                t.TF.HopsworksUdf.Name,
				"outputTypes":                         t.TF.HopsworksUdf.OutputTypes,
				"transformationFeatures":              []string{t.Column},
				"transformationFunctionArgumentNames": t.TF.HopsworksUdf.TransformationFunctionArgumentNames,
				"executionMode":                       t.TF.HopsworksUdf.ExecutionMode,
			}
			if t.TF.HopsworksUdf.DroppedArgumentNames != nil {
				udf["droppedArgumentNames"] = t.TF.HopsworksUdf.DroppedArgumentNames
			}
			if t.TF.HopsworksUdf.StatisticsArgumentNames != nil {
				udf["statisticsArgumentNames"] = t.TF.HopsworksUdf.StatisticsArgumentNames
			}

			tfEntry := map[string]interface{}{
				"id":             t.TF.ID,
				"version":        t.TF.Version,
				"featurestoreId": c.Config.FeatureStoreID,
				"hopsworksUdf":   udf,
			}
			tfList = append(tfList, tfEntry)
		}
		req["transformationFunctions"] = tfList
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	data, err := c.Post(fmt.Sprintf("%s/featureview", c.FSPath()), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	var fv FeatureView
	if err := json.Unmarshal(data, &fv); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &fv, nil
}

func (c *Client) buildFGRef(fg *FeatureGroup) map[string]interface{} {
	fgType := fg.Type
	if fgType == "" {
		fgType = "cachedFeaturegroupDTO"
		if fg.OnlineEnabled {
			fgType = "streamFeatureGroupDTO"
		}
	}
	return map[string]interface{}{
		"id":             fg.ID,
		"name":           fg.Name,
		"version":        fg.Version,
		"type":           fgType,
		"featurestoreId": c.Config.FeatureStoreID,
		"onlineEnabled":  fg.OnlineEnabled,
	}
}

func (c *Client) buildFeatureList(fg *FeatureGroup, names []string) []map[string]interface{} {
	var list []map[string]interface{}
	for _, fname := range names {
		feat := map[string]interface{}{
			"name":           fname,
			"featureGroupId": fg.ID,
		}
		for _, fgf := range fg.Features {
			if fgf.Name == fname {
				feat["type"] = fgf.Type
				feat["primary"] = fgf.Primary
				break
			}
		}
		list = append(list, feat)
	}
	return list
}

// FVQueryInfo holds the parsed query structure from a feature view.
type FVQueryInfo struct {
	BaseFG   string // "name v1"
	Features []string
	Joins    []FVQueryJoin
}

type FVQueryJoin struct {
	FGName  string
	Version int
	Type    string // LEFT, INNER, etc.
	Prefix  string
}

// GetFeatureViewQuery fetches the query definition for a feature view.
func (c *Client) GetFeatureViewQuery(name string, version int) (*FVQueryInfo, error) {
	path := fmt.Sprintf("%s/featureview/%s/version/%d/query", c.FSPath(), name, version)
	data, err := c.Get(path)
	if err != nil {
		return nil, err
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse query: %w", err)
	}

	info := &FVQueryInfo{}

	// Base FG
	if lfg, ok := raw["leftFeatureGroup"].(map[string]interface{}); ok {
		name, _ := lfg["name"].(string)
		ver, _ := lfg["version"].(float64)
		info.BaseFG = fmt.Sprintf("%s v%d", name, int(ver))
	}

	// Left features
	if lf, ok := raw["leftFeatures"].([]interface{}); ok {
		for _, f := range lf {
			if fm, ok := f.(map[string]interface{}); ok {
				if n, ok := fm["name"].(string); ok {
					info.Features = append(info.Features, n)
				}
			}
		}
	}

	// Joins (recursive to handle nested/chained joins)
	info.Joins = parseQueryJoins(raw)


	return info, nil
}

// parseQueryJoins recursively extracts joins from a query map,
// flattening nested joins into a single list.
func parseQueryJoins(query map[string]interface{}) []FVQueryJoin {
	joins, ok := query["joins"].([]interface{})
	if !ok {
		return nil
	}
	var result []FVQueryJoin
	for _, j := range joins {
		jm, ok := j.(map[string]interface{})
		if !ok {
			continue
		}
		join := FVQueryJoin{}
		if jt, ok := jm["type"].(string); ok {
			join.Type = jt
		}
		if prefix, ok := jm["prefix"].(string); ok {
			join.Prefix = prefix
		}
		if jq, ok := jm["query"].(map[string]interface{}); ok {
			if lfg, ok := jq["leftFeatureGroup"].(map[string]interface{}); ok {
				join.FGName, _ = lfg["name"].(string)
				ver, _ := lfg["version"].(float64)
				join.Version = int(ver)
			}
			// Recurse into nested joins
			result = append(result, join)
			result = append(result, parseQueryJoins(jq)...)
		} else {
			result = append(result, join)
		}
	}
	return result
}

func (c *Client) DeleteFeatureView(name string, version int) error {
	var path string
	if version > 0 {
		path = fmt.Sprintf("%s/featureview/%s/version/%d", c.FSPath(), name, version)
	} else {
		path = fmt.Sprintf("%s/featureview/%s", c.FSPath(), name)
	}
	_, err := c.Delete(path)
	return err
}
