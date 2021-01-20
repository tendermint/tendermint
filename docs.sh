#!/usr/bin/env bash

# clone tendermint
git clone https://github.com/tendermint/tendermint.git v0.34.x

cd v0.34.x && git checkout v0.34.x

cd .. && cd docs && mkdir v0.34

cp ../v0.34.x/docs/tutorials v0.34
