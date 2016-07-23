#! /bin/bash

# integrations test!
# if we pushed to STAGING or MASTER,
# run the integrations tests.

BRANCH=`git rev-parse --abbrev-ref HEAD`
echo "Current branch: $BRANCH"

if [[ "$BRANCH" == "master" || "$BRANCH" == "staging" ]]; then
	docker build -t tester -f ./test/Dockerfile .
	docker run -t tester bash /test_libs.sh
fi

