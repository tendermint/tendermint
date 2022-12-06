#!/bin/sh
set -euo pipefail

ARGS=$@
FLAGS=${ARGS:=-g 4 -d networks/nightly}

# This lists all tags that are reachable from the current commit, sorted by
# increasing semver version.
SORTED_TAGS=`git tag -l --merge HEAD  --sort="v:refname"`

# Extracts the major and minor version of the most recent -dev tagged version.
# For example, on the backport branch for v0.32, the most recent dev tag will contain v0.32.0-dev.
# v0.32 is then extracted from this tag.
BRANCH_MINOR_VERSION=`echo "$SORTED_TAGS" | grep '^v[0-9]\+\.[0-9]\+\.[0-9]\+\(-dev[0-9]*\)$' | tail -n 1 | sed -n 's/^\(v[0-9]\+\.[0-9]\+\).*/\1/p'`
LATEST_RELEASE_VERSION=`echo "$SORTED_TAGS" | grep  '^v[0-9]\+\.[0-9]\+\.[0-9]\+$' | tail -n 1`

# Check to see if the major and minor version of the latest release version are
# the same as those of the current branch. This ensures that the multi-version testing
# is only run on branches with a tagged release of the same major and minor version
# and not on branches that do not yet have releases such as new backport branches
# and the main branch.
if [ `echo "$LATEST_RELEASE_VERSION" | grep "^$BRANCH_MINOR_VERSION"` ]; then
	FLAGS="$FLAGS --multi-version $LATEST_RELEASE_VERSION"
fi

./build/generator $FLAGS
