#!/bin/bash

cp -a ../rpc/openapi/ .vuepress/public/rpc/
git clone https://github.com/tendermint/spec.git specRepo && cp -r specRepo/spec . && rm -rf specRepo

# copy all tendermint docs realted info into a master folder
mkdir master

mv introduction master
mv app-dev master
mv imgs master
mv networks master
mv tools master
mv tendermint-core master
mv tutorials master
mv nodes master
mv rfc master
mv README.md master

# # clone tendermint for use of multiple versions
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
