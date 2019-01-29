#!/bin/sh
set -e

for CFG_FILE in $(find /tmp/nodes -name 'config.toml'); do
    NODE_ID=$(basename $(dirname $(dirname ${CFG_FILE})))
    cat ${CFG_FILE} | \
        sed "s/t\([ioa]\)k\([0-9]*\):/t\1k\2.sredev.co:/g" | \
        sed "s/^proxy_app = \(.*\)$/proxy_app = \"kvstore\"/" | \
        sed "s/^moniker = \(.*\)$/moniker = \"${NODE_ID}\"/" | \
        sed "s/^log_format = \(.*\)$/log_format = \"json\"/" | \
        sed "s/^create_empty_blocks = \(.*\)$/create_empty_blocks = false/" | \
        sed "s/^prometheus = \(.*\)$/prometheus = true/" \
        > ${CFG_FILE}
    echo "Rewrote \"config.toml\" for ${NODE_ID}"
done
