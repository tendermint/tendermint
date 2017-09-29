#!/bin/bash

HOME=$1

tendermint init --home $HOME
echo "
[consensus]
timeout_commit = 0
" >> $HOME/config.toml

echo "starting validator node"
tendermint node \
  --home $HOME \
  --proxy_app dummy \
  --p2p.laddr tcp://127.0.0.1:56656 \
  --rpc.laddr tcp://127.0.0.1:56657 \
  --log_level error &

echo "making blocks for 60s"
sleep 60

kill %1
