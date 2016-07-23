#! /bin/bash

# integrations test
# this is the script run by eg CircleCI.
# It creates a docker container,
# installs the dependencies,
# and runs the tests.
# If we pushed to STAGING or MASTER,
# it will also run the tests for all dependencies

docker build -t tester -f ./test/Dockerfile .
docker run -t tester bash test/run_test.sh
