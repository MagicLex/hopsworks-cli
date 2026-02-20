# Snowflake Test Credentials

| Field | Value |
|-------|-------|
| Account identifier | WBGBPNF-KQ07314 |
| Account/Server URL | WBGBPNF-KQ07314.snowflakecomputing.com |
| Login name | SNOWPLEX |
| Role | ACCOUNTADMIN |
| Cloud platform | AWS |
| Edition | Enterprise |

## Verified CLI Flow

```bash
# 1. Create connector
hops connector create snowflake test_sf \
  --url "https://wbgbpnf-kq07314.snowflakecomputing.com" \
  --user SNOWPLEX --password '<password>' \
  --database SNOWFLAKE_SAMPLE_DATA --schema PUBLIC --warehouse COMPUTE_WH \
  --role ACCOUNTADMIN --description "Snowflake test connector"

# 2. Test connection
hops connector test test_sf
# -> Connected to test_sf (SNOWFLAKE)
# -> Found 3 databases: SNOWFLAKE, SNOWFLAKE_SAMPLE_DATA, SNOWFLAKE_LEARNING_DB

# 3. Browse
hops connector databases test_sf
hops connector tables test_sf --database SNOWFLAKE_SAMPLE_DATA
hops connector preview test_sf --database SNOWFLAKE_SAMPLE_DATA --table CUSTOMER --schema TPCH_SF001

# 4. Create external FGs (explicit features — Snowflake needs UPPERCASE names)
hops fg create-external snowflake__orders \
  --connector test_sf \
  --query "SELECT O_ORDERKEY, O_CUSTKEY, O_ORDERSTATUS, O_TOTALPRICE, O_ORDERPRIORITY, O_SHIPPRIORITY FROM SNOWFLAKE_SAMPLE_DATA.TPCH_SF001.ORDERS" \
  --features "O_ORDERKEY:bigint,O_CUSTKEY:bigint,O_ORDERSTATUS:string,O_TOTALPRICE:double,O_ORDERPRIORITY:string,O_SHIPPRIORITY:bigint" \
  --primary-key O_ORDERKEY \
  --description "Orders from Snowflake TPC-H (external)"

hops fg create-external snowflake__customers \
  --connector test_sf \
  --query "SELECT C_CUSTKEY, C_NATIONKEY, C_ACCTBAL, C_MKTSEGMENT FROM SNOWFLAKE_SAMPLE_DATA.TPCH_SF001.CUSTOMER" \
  --features "C_CUSTKEY:bigint,C_NATIONKEY:bigint,C_ACCTBAL:double,C_MKTSEGMENT:string" \
  --primary-key C_CUSTKEY \
  --description "Customers from Snowflake TPC-H (external)"

# 5. Preview data through Hopsworks
hops fg preview snowflake__orders --n 5
hops fg preview snowflake__customers --n 5
```

## Gotchas

- **Uppercase identifiers**: Snowflake defaults to UPPERCASE. Feature names and query columns must match casing.
- **Auto-infer grabs full table**: When using `--database/--table`, inferred schema is filtered to columns in the `--query` SELECT clause.
- **Stats disabled**: External FGs skip statistics computation (Spark can't reach Snowflake from the cluster).
- **Preview endpoint**: The FG preview calls Snowflake in real-time via the Hopsworks backend's Arrow Flight connector.
