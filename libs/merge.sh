#! /bin/bash
set -e

# NOTE: go-alert depends on go-common

REPOS=("autofile" "clist" "db" "events" "flowrate" "logger" "process")

mkdir common
git mv *.go common
git mv LICENSE common

git commit -m "move all files to common/ to begin repo merge"

for repo in "${REPOS[@]}"; do 
	# add and fetch the repo
	git remote add -f "$repo" "https://github.com/tendermint/go-${repo}"

	# merge master and move into subdir
	git merge "$repo/master" --no-edit

	if [[ "$repo" != "flowrate" ]]; then
		mkdir "$repo"
		git mv *.go "$repo/"
	fi

	set +e # these might not exist
        git mv *.md "$repo/"
	git mv README "$repo/README.md"
	git mv Makefile "$repo/Makefile"
        git rm LICENSE
	set -e
        
	# commit
	git commit -m "merge go-${repo}"

	git remote rm "$repo"
done

go get github.com/ebuchman/got
got replace "tendermint/go-common" "tendermint/go-common/common"
for repo in "${REPOS[@]}"; do 

	if [[ "$repo" != "flowrate" ]]; then
		got replace "tendermint/go-${repo}" "tendermint/go-common/${repo}"
	else
		got replace "tendermint/go-${repo}/flowrate" "tendermint/go-common/flowrate"
	fi
done

git add -u 
git commit -m "update import paths"

# TODO: change any paths in non-Go files
# TODO: add license
