#!/bin/sh
set -e

START_TOXIPROXY_PORT=${START_TOXIPROXY_PORT:-34000}
NODE_DOMAIN=${NODE_DOMAIN:-"sredev.co"}

for CFG_FILE in $(find /tmp/nodes -name 'config.toml'); do
    NODE_ID=$(basename $(dirname $(dirname ${CFG_FILE})))
    # calculate the port to which to connect to this node through Toxiproxy
    NODE_TOXIPROXY_PORT=$((${START_TOXIPROXY_PORT}+${NODE_ID//[^0-9]/}))
    sed -i '' \
        -e "s|^laddr = \"tcp://0.0.0.0:26656\"$|laddr = \"tcp://127.0.0.1:26656\"|" \
        -e "s|^laddr = \"tcp://0.0.0.0:26657\"$|laddr = \"tcp://127.0.0.1:26657\"|" \
        -e "s|^external_address = \(.*\)$|external_address = \"tcp://${NODE_ID}.${NODE_DOMAIN}:${NODE_TOXIPROXY_PORT}\"|" \
        -e "s/t\([ioa]\)k\([0-9]*\):/t\1k\2.${NODE_DOMAIN}:/g" \
        -e "s/^proxy_app = \(.*\)$/proxy_app = \"kvstore\"/" \
        -e "s/^moniker = \(.*\)$/moniker = \"${NODE_ID}\"/" \
        -e "s/^log_format = \(.*\)$/log_format = \"json\"/" \
        -e "s/^create_empty_blocks = \(.*\)$/create_empty_blocks = false/" \
        -e "s/^prometheus = \(.*\)$/prometheus = true/" \
        ${CFG_FILE}
    echo "Rewrote \"config.toml\" for ${NODE_ID}"
done
