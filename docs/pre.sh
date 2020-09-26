#!/bin/bash

cp -a ../rpc/openapi/ .vuepress/public/rpc/

git clone https://github.com/tendermint/spec.git specRepo
cp -r specRepo/spec .
rm -rf specRepo

mkdir 0.32
git clone -b v0.32.0  --depth 1  https://github.com/tendermint/tendermint.git 32Repo
cp -r 32Repo/docs 0.32
rm -rf 32Repo
rm -rf 0.32/docs/DOCS_README.md
rm -rf 0.32/docs/README.md
cd 0.32/docs
mv * ../
rm -rf 0.32/docs

cd ../..

mkdir 0.33
git clone -b v0.33.0  --depth 1  https://github.com/tendermint/tendermint.git 33Repo
cp -r 33Repo/docs 0.33
rm -rf 33Repo
rm -rf 0.33/docs/DOCS_README.md
rm -rf 0.33/docs/README.md
cd 0.33/docs
mv * ../
rm -rf 0.33/docs