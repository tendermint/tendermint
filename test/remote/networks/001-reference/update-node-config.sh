#!/bin/sh
set -e

NODE_DOMAIN=${NODE_DOMAIN:-"sredev.co"}

for CFG_FILE in $(find /tmp/nodes -name 'config.toml'); do
    NODE_ID=$(basename $(dirname $(dirname ${CFG_FILE})))
    sed -i '' \
        -e "s/t\([ioa]\)k\([0-9]*\):/t\1k\2.${NODE_DOMAIN}:/g" \
        -e "s/^proxy_app = \(.*\)$/proxy_app = \"kvstore\"/" \
        -e "s/^moniker = \(.*\)$/moniker = \"${NODE_ID}\"/" \
        -e "s/^log_format = \(.*\)$/log_format = \"json\"/" \
        -e "s/^create_empty_blocks = \(.*\)$/create_empty_blocks = false/" \
        -e "s/^prometheus = \(.*\)$/prometheus = true/" \
        ${CFG_FILE}
    echo "Rewrote \"config.toml\" for ${NODE_ID}"
done
