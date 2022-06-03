#!/bin/sh
#
# Invoke Mockery v2 to update generated mocks for the given type.
#
# This script runs a locally-installed "mockery" if available, otherwise it
# runs the published Docker container. This legerdemain is so that the CI build
# and a local build can work off the same script.
#
if ! which mockery ; then
  mockery() {
    docker run --rm -v "$PWD":/w --workdir=/w vektra/mockery:v2.12.3
  }
fi

mockery --disable-version-string --case underscore --name "$@"
