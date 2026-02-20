# Job Management — Findings & API Quirks

Notes from building full job lifecycle support (`hops job {create,info,run,stop,logs,history,delete}`).

## 1. Two Default Config Endpoints

Hopsworks has **two** endpoints for job configuration defaults:

| Endpoint | Purpose | Returns |
|----------|---------|---------|
| `GET /projects/{pid}/jobs/{type}/configuration` | System default for job type | Always works, returns base config |
| `GET /projects/{pid}/jobconfig/{TYPE}` | User-saved default per project | 404 if user never saved one |

The `{type}` parameter is **lowercase** for the first (`python`, `pyspark`, `spark`, `ray`) and **UPPERCASE** for the second (`PYTHON`, `PYSPARK`).

The CLI tries the system endpoint first, falls back to user-saved.

**Source**: `JobsResource.java` line 336 (`{jobtype : python|docker|spark|pyspark|flink|ray|ingestion}/configuration`) and `DefaultJobConfigurationResource.java`.

## 2. PUT /jobs/{name} Takes Config Directly, Not a Job Object

The PUT endpoint for creating/updating a job takes a `JobConfiguration` as the request body — **not** a Job wrapper with nested config.

```
# WRONG — 422 "Job configuration was not provided"
PUT /projects/{pid}/jobs/myjob
{"name": "myjob", "config": {"type": "pythonJobConfiguration", ...}}

# CORRECT
PUT /projects/{pid}/jobs/myjob
{"type": "pythonJobConfiguration", "appName": "myjob", "appPath": "...", ...}
```

The `name` comes from the URL path. The body IS the config.

**Source**: `JobsResource.java` line 184: `public Response put(JobConfiguration config, @PathParam("name") String name, ...)`

## 3. PySpark/Spark Need `jobType` and `mainClass`

The system default config for `pyspark` returns `type: sparkJobConfiguration` but **no** `jobType` field. Without it, the backend crashes:

```
500: Cannot invoke "JobType.ordinal()" because "jobType" is null
```

Required fields the CLI must inject:
- `jobType`: `"PYSPARK"` or `"SPARK"` (the default config only has the Jackson discriminator `type`, not the enum)
- `mainClass`: `"org.apache.spark.deploy.PythonRunner"` for PySpark
- `appName`: must match the URL path param

## 4. App Path Format Differs by Job Type

| Job Type | appPath format | Example |
|----------|---------------|---------|
| PYTHON | Relative project path | `Resources/jobs/script.py` |
| PYSPARK | Full HDFS path | `hdfs:///Projects/lexterm/Resources/jobs/script.py` |
| SPARK | Full HDFS path | `hdfs:///Projects/lexterm/Resources/jars/app.jar` |

Python jobs accept relative paths and resolve them internally. Spark/PySpark jobs need the full `hdfs:///` URI — a relative path causes the job to fail during INITIALIZING with **no logs** (the Spark container never starts).

## 5. Execution Lifecycle States

```
INITIALIZING → RUNNING → AGGREGATING_LOGS → FINISHED
                                           → FAILED
                       → KILLED
```

Terminal states: `FINISHED`, `FAILED`, `KILLED`, `FRAMEWORK_FAILURE`, `APP_MASTER_START_FAILED`, `INITIALIZATION_FAILED`.

Jobs that fail during `INITIALIZING` produce **no stdout/stderr logs** — the container never ran.

## 6. Stop Execution Payload

```
PUT /projects/{pid}/jobs/{name}/executions/{id}/status
{"state": "stopped"}
```

Not `"STOPPED"` (uppercase) — must be lowercase `"stopped"`.

## 7. Run Execution — text/plain Body

```
POST /projects/{pid}/jobs/{name}/executions
Content-Type: text/plain

optional args string here
```

The body is plain text (not JSON), used as argument override. Empty body = use job's `defaultArgs`.

**Source**: `ExecutionsResource.java`

## 8. Default Environment Names

| Job Type | Default Environment |
|----------|-------------------|
| PYTHON | `pandas-training-pipeline` |
| PYSPARK/SPARK | `spark-feature-pipeline` |

These are set in `Settings.java`:
- `DEFAULT_PYTHON_JOB_ENVIRONMENT("pandas-training-pipeline")`
- `DOCKER_BASE_IMAGE_SPARK` → `spark-feature-pipeline`

## 9. Config Type Discriminators (Jackson)

| Job Type | `config.type` value |
|----------|-------------------|
| PYTHON | `pythonJobConfiguration` |
| PYSPARK | `sparkJobConfiguration` |
| SPARK | `sparkJobConfiguration` |
| RAY | `rayJobConfiguration` |
| DOCKER | `dockerJobConfiguration` |

PySpark and Spark share `sparkJobConfiguration` — they're distinguished by `config.jobType` (`PYSPARK` vs `SPARK`) and `mainClass`.

## 10. Verified Job Lifecycle (Live Testing)

### Python Job
```bash
hops job create testhello --type python --app-path Resources/jobs/test_hello.py
# → Created job 'testhello' (ID: 55, type: python)

hops job run testhello --wait --poll 5
# → Started execution #39, FINISHED SUCCEEDED in 16s

hops job logs testhello
# → "Hello from hops job create test"

hops job delete testhello
```

### PySpark Job
```bash
hops job create testpyspark --type pyspark \
  --app-path "hdfs:///Projects/lexterm/Resources/jobs/test_pyspark.py"
# → Created job 'testpyspark' (ID: 57, type: pyspark)

hops job run testpyspark --wait --poll 10
# → Started execution #41, FINISHED SUCCEEDED in 55s

hops job logs testpyspark
# → Spark DataFrame output: |id|msg| |1|hello| |2|world|

hops job delete testpyspark
```

## 11. Ray Config Field Names

The default config endpoint returns these field names (different from what you might guess):

```json
{
  "type": "rayJobConfiguration",
  "driverCores": 1.0,
  "driverMemory": 2048,
  "workerMinInstances": 1,    // NOT "minWorkers"
  "workerMaxInstances": 1,    // NOT "maxWorkers"
  "workerCores": 1.0,
  "workerMemory": 4096,
  "workerGpus": 0,
  "driverGpus": 0,
  "jobType": "RAY"
}
```

The default config endpoint (`/jobs/ray/configuration`) requires the `ray-training-pipeline` environment to exist in the project. If it doesn't, you get `404: Could not find the environment`. Create it first via the Python SDK: `project.get_environment_api().create_environment('ray-training-pipeline')`.

**This cluster**: Ray jobs are disabled (`400: Ray jobs are not enabled in this cluster`). The CLI wiring is correct but untestable here. Images exist (`preset-images-*-ray-*`) but the feature flag is off.

## 12. Pure Spark (JAR) Jobs

The CLI supports `--type spark --main-class <class>` for pure Scala/Java Spark jobs.

Hopsworks Spark image (3.5.5) **doesn't ship `spark-examples` JAR** — production build, examples stripped. Downloaded from `https://repo.hops.works/master/spark-examples_2.12-3.1.1.5.jar` (only version available) and uploaded to HopsFS.

```bash
# Download and upload
curl -sL "https://repo.hops.works/master/spark-examples_2.12-3.1.1.5.jar" \
  -o /hopsfs/Resources/jars/spark-examples_2.12-3.1.1.5.jar

# Create and run — HDFS path required for Spark
hops job create sparkpi --type spark \
  --app-path "hdfs:///Projects/lexterm/Resources/jars/spark-examples_2.12-3.1.1.5.jar" \
  --main-class "org.apache.spark.examples.SparkPi"

hops job run sparkpi --wait
# → FINISHED SUCCEEDED in 44s
# → "Pi is roughly 3.145395726978635"
```

Despite Spark 3.1.1 JAR on Spark 3.5.5 runtime, SparkPi ran successfully (backwards compatible).

## 13. Job Scheduling (v2 Cron API)

Hopsworks has two schedule systems:
- **Legacy** (`PUT /jobs/{name}/schedule`): interval-based (number + unit). Deprecated.
- **v2** (`/jobs/{name}/schedule/v2`): cron-based with CRUD. This is what we use.

**Cron format**: Quartz 6-field: `SEC MIN HOUR DAY MONTH WEEKDAY` (use `?` for unspecified day/weekday).

**Timestamps**: epoch milliseconds (not ISO strings). The `InstantAdapter` serializes `java.time.Instant` as `Long`.

**Update quirk**: `PUT` requires the schedule `id` in the body. The CLI handles this automatically — on update, it fetches the existing schedule to get the ID, then sends the update.

```bash
hops job schedule myjob "0 0 * * * ?"              # every hour
hops job schedule myjob "0 */15 * * * ?"            # every 15 min (auto-updates if exists)
hops job schedule-info myjob                        # show cron + next execution
hops job unschedule myjob                           # remove schedule
```

## Commands Added

```
hops job list                          # existing
hops job status <name> [--wait]        # existing
hops job info <name>                   # NEW — show config
hops job create <name> --type --app-path [flags]  # NEW
hops job run <name> [--args] [--wait]  # NEW
hops job stop <name> [--exec ID]       # NEW
hops job logs <name> [--exec ID] [--type out|err]  # NEW
hops job history <name> [--limit N]    # NEW
hops job delete <name>                 # NEW
hops job schedule <name> <cron> [--start --end]  # NEW — cron scheduling
hops job schedule-info <name>          # NEW — show schedule
hops job unschedule <name>             # NEW — remove schedule
```
