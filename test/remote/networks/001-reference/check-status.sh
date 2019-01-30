#!/bin/sh

EXIT_CODE=0

echo "Waiting a few seconds for Tendermint network to start up..."
sleep 3

# Run through each node and check its status to see if it's up
for CFG_FILE in $(find /tmp/nodes -name 'config.toml'); do
    NODE_ID=$(basename $(dirname $(dirname ${CFG_FILE})))
    if curl -s -m 3 "http://${NODE_ID}.sredev.co:26657/status" > /dev/null; then
        echo "${NODE_ID} is UP"
    else
        echo "${NODE_ID} is DOWN"
        EXIT_CODE=1
    fi
done

exit ${EXIT_CODE}
