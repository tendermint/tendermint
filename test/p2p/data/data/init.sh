#! /bin/bash
# This is a sample bash script for MerkleEyes.
# NOTE: mintnet expects data.sock to be created

go get github.com/tendermint/merkleeyes/cmd/merkleeyes

merkleeyes server --address="unix:///data/tendermint/data/data.sock"