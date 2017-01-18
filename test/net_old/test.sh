#! /bin/bash
set -eu

# install mintnet, netmon, fetch network_testing
bash test/net/setup.sh

# start the testnet
bash test/net/start.sh
