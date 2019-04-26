#!/bin/sh

# github-public-newbranch.bash - create public branch from the security repository

set -euo pipefail

# Create new branch
BRANCH="${CIRCLE_TAG:-v0.0.0}-security-`date -u +%Y%m%d%H%M%S`"
# Check if the patch release exist already as a branch
if [ -n "`git branch | grep '${BRANCH}'`" ]; then
  echo "WARNING: Branch ${BRANCH} already exists."
else
  echo "Creating branch ${BRANCH}."
  git branch "${BRANCH}"
fi

# ... and check it out
git checkout "${BRANCH}"

# Add entry to public repository
git remote add tendermint-origin git@github.com:tendermint/tendermint.git

# Push branch and tag to public repository
git push tendermint-origin
git push tendermint-origin --tags

# Create a PR from the public branch to the assumed release branch in public (release branch has to exist)
python -u scripts/release_management/github-openpr.py --head "${BRANCH}" --base "${BRANCH:%.*}"
