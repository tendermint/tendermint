#!/bin/sh
set -e

NODE_DOMAIN=${NODE_DOMAIN:-"sredev.co"}

for CFG_FILE in $(find /tmp/nodes -name 'config.toml'); do
    NODE_ID=$(basename $(dirname $(dirname ${CFG_FILE})))
    sed -i '' \
        -e "s/t\([ioa]\)k\([0-9]*\):/t\1k\2.${NODE_DOMAIN}:/g" \
        -e "s/^proxy_app = \(.*\)$/proxy_app = \"kvstore\"/" \
        -e "s/^moniker = \(.*\)$/moniker = \"${NODE_ID}\"/" \
        -e "s/^db_backend = \(.*\)$/db_backend = \"goleveldb\"/" \
        -e "s/^log_level = \(.*\)$/log_level = \"main:info,state:info,consensus:info,*:error\"/" \
        -e "s/^log_format = \(.*\)$/log_format = \"json\"/" \
        -e "s/^addr_book_strict = \(.*\)$/addr_book_strict = true/" \
        -e "s/^flush_throttle_timeout = \(.*\)$/flush_throttle_timeout = \"10ms\"/" \
        -e "s/^max_packet_msg_payload_size = \(.*\)$/max_packet_msg_payload_size = 10240/" \
        -e "s/^send_rate = \(.*\)$/send_rate = 20971520/" \
        -e "s/^recv_rate = \(.*\)$/recv_rate = 20971520/" \
        -e "s/^recheck = \(.*\)$/recheck = false/" \
        -e "s/^size = \(.*\)$/size = 50000/" \
        -e "s/^cache_size = \(.*\)$/cache_size = 100000/" \
        -e "s/^create_empty_blocks = \(.*\)$/create_empty_blocks = false/" \
        -e "s/^prometheus = \(.*\)$/prometheus = true/" \
        ${CFG_FILE}
    echo "Rewrote \"config.toml\" for ${NODE_ID}"
done
