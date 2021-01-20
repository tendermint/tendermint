#!/usr/bin/env bash

# copy all tendermint docs realted info into a master folder
mkdir master

cp -r introduction master
cp -r app-dev master
cp -r imgs master
cp -r networks master
cp -r tools master
cp -r tendermint-core master
cp -r tutorials master
cp -r nodes master
cp -r rfc master
cp -r README.md master

# # clone tendermint
# todo: merge the two versions below when 0.35 is released
git clone https://github.com/tendermint/tendermint.git doc-versions

cd doc-versions && git checkout v0.34.x

cd .. && mkdir v0.34

cp -r doc-versions/docs/tutorials v0.34
cp -r doc-versions/docs/tendermint-core v0.34
cp -r doc-versions/docs/introduction v0.34
cp -r doc-versions/docs/app-dev v0.34
cp -r doc-versions/docs/imgs v0.34
cp -r doc-versions/docs/tools v0.34
cp -r doc-versions/docs/networks v0.34
cp -r doc-versions/docs/README.md v0.34

cd doc-versions && git checkout v0.33.x

cd .. && mkdir v0.33

cp -r doc-versions/docs/tendermint-core v0.33
cp -r doc-versions/docs/introduction v0.33
cp -r doc-versions/docs/app-dev v0.33
cp -r doc-versions/docs/imgs v0.33
cp -r doc-versions/docs/tools v0.33
cp -r doc-versions/docs/networks v0.33
cp -r doc-versions/docs/guides v0.33
cp -r doc-versions/docs/README.md v0.33

rm -rf doc-versions
