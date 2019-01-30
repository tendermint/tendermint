# Distributed `kvstore` Load Test

This scenario is similar to
[`002-kvstore-loadtest`](../002-kvstore-loadtest/README.md) in that it attempts
to generate a reasonable amount of load by hitting the HTTP interface of the
deployed Tendermint nodes, assuming that the `kvstore` proxy app is running. It
does so, however, from multiple source machines.

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
# The load test is executed from the following 4 machines:
# - tok0.sredev.co (master)
# - tok1.sredev.co (slave)
# - tok2.sredev.co (slave)
# - tok3.sredev.co (slave)
make scenario:003-kvstore-loadtest-distributed
```

## Statistics
After load test execution, the stdout output, log file and CSV statistics files
for each machine will be fetched and placed into the
`/tmp/003-kvstore-loadtest-distributed/{node_id}` folders.
