#! /bin/bash
# This is a sample bash script for a ABCI application

cd app/
git clone https://github.com/tendermint/nomnomcoin.git
cd nomnomcoin
npm install .

node app.js --eyes="unix:///data/tendermint/data/data.sock"