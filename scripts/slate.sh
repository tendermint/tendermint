#!/usr/bin/env bash
set -euo pipefail

if [ "$CIRCLE_BRANCH" == "" ]; then
	echo "this script is meant to be run on CircleCI, exiting"
	echo 1
fi

# check for changes in the `rpc/core` directory
did_rpc_change=$(git diff --name-status $CIRCLE_BRANCH develop | grep rpc/core)

if [ "$did_rpc_change" == "" ]: then
	echo "no changes detected in rpc/core, exiting"
	exit 0
else
	echo "changes detected in rpc/core, continuing"
fi

# only run this script on changes to rpc/core committed to develop
if [ "$CIRCLE_BRANCH" != "develop" ]; then
	echo "the branch being built isn't develop, exiting"
	exit 0
else
	echo "on develop, building the RPC docs"
fi

# godoc2md used to convert the go documentation from
# `rpc/core` into a markdown file consumed by Slate
go get github.com/melekes/godoc2md

# slate works via forks, and we'll be committing to
# master branch, which will trigger our fork to run
# the `./deploy.sh` and publish via the `gh-pages` branch
slate_repo=github.com/tendermint/slate
slate_path="$GOPATH"/src/"$slate_repo"

if [ ! -d "$slate_path" ]; then
	git clone https://"$slate_repo".git $slate_path
fi

# the main file we need to update if rpc/core changed
destination="$slate_path"/source/index.html.md

# we remove it then re-create it with the latest changes
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

# generate a markdown from the godoc comments, using a template
rpc_docs=$(godoc2md -template rpc/core/doc_template.txt github.com/tendermint/tendermint/rpc/core | grep -v -e "pipe.go" -e "routes.go" -e "dev.go" | sed 's$/src/target$https://github.com/tendermint/tendermint/tree/master/rpc/core$')

# append core RPC docs
echo "$rpc_docs" >> "$destination"

# commit the changes
cd $slate_path

git config --global user.email "github@tendermint.com"
git config --global user.name "tenderbot"

git commit -a -m "Update tendermint RPC docs via CircleCI"
git push -q https://${GITHUB_ACCESS_TOKEN}@github.com/tendermint/slate.git master
