# Snowflake Test Credentials

Username: SNOWPLEX
Dedicated Login URL: https://wbgbpnf-kq07314.snowflakecomputing.com
Account Identifier: wbgbpnf-kq07314

## CLI Test Commands

```bash
# Create connector
hops connector create snowflake my_sf \
  --url "https://wbgbpnf-kq07314.snowflakecomputing.com" \
  --user SNOWPLEX \
  --password "<password>" \
  --database "<db>" --schema PUBLIC --warehouse "<wh>"

# Test connection
hops connector test my_sf

# Browse
hops connector databases my_sf
hops connector tables my_sf --database <db>
```
