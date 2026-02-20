# Feature Store REST API Reference

Base path: `/hopsworks-api/api/project/{projectId}/featurestores/{fsId}`

## Feature Groups

| Method | Path | Description |
|--------|------|-------------|
| GET | `/featuregroups` | List all FGs |
| GET | `/featuregroups/{name}?version=N` | Get FG by name (returns array) |
| POST | `/featuregroups` | Create FG (type: cachedFeaturegroupDTO or onDemandFeaturegroupDTO) |
| DELETE | `/featuregroups/{id}` | Delete FG |
| GET | `/featuregroups/{id}/preview?storage=offline&limit=N` | Preview data |
| GET | `/featuregroups/{id}/statistics?fields=content` | Get statistics |
| POST | `/featuregroups/{id}/statistics/compute` | Trigger stats computation (Spark) |
| GET | `/featuregroups/{id}/commits` | List Hudi commits |

### Statistics query params
- `fields=content` — **required** to get actual stats (not just metadata)
- `feature_names=col1,col2` — filter to specific features
- `sort_by=computation_time:desc` — sort by computation time
- `filter_by=ROW_PERCENTAGE_EQ:1.0` — filter by row percentage
- `offset=0&limit=1` — pagination

### FeatureDescriptiveStatistics fields
| Field | Type | When |
|-------|------|------|
| featureName | string | always |
| featureType | string | always (Boolean/Fractional/Integral/String) |
| count | int64 | any |
| completeness | float32 | any |
| numNullValues | int64 | any |
| min, max, mean, stddev, sum | float64 | numerical |
| approxNumDistinctValues | int64 | any |
| distinctness, entropy, uniqueness | float32 | with exact uniqueness |

## Feature Views

| Method | Path | Description |
|--------|------|-------------|
| GET | `/featureview` | List all FVs |
| GET | `/featureview/{name}/version/{version}` | Get specific FV |
| POST | `/featureview` | Create FV (with QueryDTO) |
| DELETE | `/featureview/{name}/version/{version}` | Delete specific version |
| DELETE | `/featureview/{name}` | Delete all versions |

## Training Datasets

| Method | Path | Description |
|--------|------|-------------|
| GET | `/featureview/{name}/version/{ver}/trainingdatasets` | List TDs |
| POST | `/featureview/{name}/version/{ver}/trainingdatasets` | Create TD |
| DELETE | `/featureview/{name}/version/{ver}/trainingdatasets/{tdVer}` | Delete TD |

## Storage Connectors

| Method | Path | Description |
|--------|------|-------------|
| GET | `/storageconnectors` | List connectors |

## Transformation Functions

| Method | Path | Description |
|--------|------|-------------|
| GET | `/transformationfunctions` | List transformation functions |
