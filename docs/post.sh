#!/bin/bash

rm -rf ./.vuepress/public/rpc
rm -rf ./spec
rm -rf v0.33
rm -rf v0.34

# mv master docs back into place
mv master/introduction .
mv master/app-dev .
mv master/imgs .
mv master/networks .
mv master/tools .
mv master/tendermint-core .
mv master/tutorials .
mv master/nodes .
mv master/rfc .
mv master/README.md .

rm -rf master
