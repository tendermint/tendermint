#! /bin/bash

echo `pwd`

BRANCH=`git rev-parse --abbrev-ref HEAD`
echo "Current branch: $BRANCH"

make get_vendor_deps

make test_race

if [[ "$BRANCH" == "master" || "$BRANCH" == "staging" ]]; then
	bash test/test_libs.sh
fi
