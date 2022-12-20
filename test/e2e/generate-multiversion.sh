#!/bin/bash
set -euo pipefail

ARGS=$@
FLAGS=${ARGS:=-g 4 -d networks/nightly}

# This lists all tags that are reachable from the current commit, sorted by
# increasing semver version.
SORTED_TAGS=`git tag -l --merge HEAD  --sort="v:refname"`

# Extracts the major and minor version of the most recent dev tagged version.
# Newer dev branches are tagged dev-<version> whereas older dev branches are tagged
# <version>-dev. This script first tries to find the highest version tagged with dev-<version>.
# For example, on the backport branch for v0.32, the most recent dev tag will contain v0.32.0-dev.
# v0.32 is then extracted from this tag. On the branch for v0.38, which uses the newer
# tag style, the tag will contain dev-v0.38

# First check for the newer style tag.
BRANCH_MINOR_VERSION=`echo "$SORTED_TAGS" | grep '^dev-v[0-9]\+\.[0-9]\+\.[0-9]\+$' | tail -n 1 | sed -n 's/^dev-\(v[0-9]\+\.[0-9]\+\)\.[0-9]\+/\1/p'`

# If we found no dev-<version> tag, then try to find the highest <version>-dev version instead.
if [ -z "$BRANCH_MINOR_VERSION" ]; then
	BRANCH_MINOR_VERSION=`echo "$SORTED_TAGS" | grep '^v[0-9]\+\.[0-9]\+\.[0-9]\+\(-dev[0-9]\+\)$' | tail -n 1 | sed -n 's/^\(v[0-9]\+\.[0-9]\+\).*/\1/p'`
fi
LATEST_RELEASE_VERSION=`echo "$SORTED_TAGS" | grep  '^v[0-9]\+\.[0-9]\+\.[0-9]\+$' | tail -n 1`

# Check to see if the major and minor version of the latest released version are
# the same as those of the current branch. This ensures that the multi-version testing
# is only run on branches with a tagged release of the same major and minor version
# and not on branches that do not yet have releases such as new backport branches
# and the main branch.
if [ `echo "$LATEST_RELEASE_VERSION" | grep "^$BRANCH_MINOR_VERSION"` ]; then
	FLAGS="$FLAGS --multi-version $LATEST_RELEASE_VERSION"
fi

./build/generator $FLAGS
