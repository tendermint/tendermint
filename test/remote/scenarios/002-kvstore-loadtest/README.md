# Single-Machine `kvstore` Load Test

This scenario attempts to generate a reasonable amount of load by hitting the
HTTP interface of the deployed Tendermint nodes, assuming that the `kvstore`
proxy app is running.

## Execution
To execute the load test from a single machine:

```bash
# Optional: deploy a clean reference network
make deploy:001-reference

# Standard load test (1000 clients, spawning 20 clients/sec, running for 60s)
# By default, this hits the following URLs:
# - http://tik0.sredev.co:26657
# - http://tik1.sredev.co:26657
# - http://tik2.sredev.co:26657
# - http://tik3.sredev.co:26657
make scenario:002-kvstore-loadtest

# Custom parameters for load test.
# This eventually spawns up to 2000 clients, spawning at 40 clients/sec,
# running for a total time of 10 minutes.
# Note that host URLs are separated by double colons (::).
HOST_URLS=http://tik2.sredev.co:26657::http://tik3.sredev.co:26657 \
    NUM_CLIENTS=2000 \
    HATCH_RATE=40 \
    RUN_TIME=10m \
    make scenario:002-kvstore-loadtest
```

## Statistics
Load statistics will be shown on `stdout`, and the following files will be
generated:

* `/tmp/002-kvstore-loadtest.log` - The log file detailing specific errors or
  issues that occurred during load testing.
* `/tmp/002-kvstore-loadtest_distribution.csv` - A distribution of the response
  times for each class of query.
* `/tmp/002-kvstore-loadtest_requests.csv` - Request statistics for each class
  of query.
