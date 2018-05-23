#!/usr/bin/env bash
set -euo pipefail

if [ "$CIRCLE_BRANCH" != "develop" ]; then
	exit 0
fi

# godoc2md used to convert the go documentation from
# `rpc/core` into a markdown file consumed by Slate
go get github.com/melekes/godoc2md

# slate works via forks, so we'll be committing to
# the `gh-pages` branch
slate_repo=github.com/tendermint/slate
slate_path="$GOPATH"/src/"$slate_repo"

if [ ! -d "$slate_path" ]; then
	git clone https://"$slate_repo".git $slate_path
fi

destination="$slate_path"/source/index.html.md

rm $destination

header="---
title: RPC Reference

language_tabs:
  - shell
  - go

toc_footers:
  - <a href='https://tendermint.com/'>Tendermint</a>
  - <a href='https://github.com/lord/slate'>Documentation Powered by Slate</a>

search: true
---"

# write header to the main slate file
echo "$header" > "$destination"

rpc_docs=$(godoc2md -template rpc/core/doc_template.txt github.com/tendermint/tendermint/rpc/core | grep -v -e "pipe.go" -e "routes.go" -e "dev.go" | sed 's$/src/target$https://github.com/tendermint/tendermint/tree/master/rpc/core$')

# append core RPC docs
echo "$rpc_docs" >> "$destination"

# commit the changes
# need a special user for the CI
cd $slate_path

git config --global user.email "github@tendermint.com"
git config --global user.name "tenderbot"

git commit -a -m "Update tendermint RPC docs via CircleCI"
git push -q https://${GITHUB_ACCESS_TOKEN}@github.com/tendermint/slate.git master
