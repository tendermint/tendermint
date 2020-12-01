#!/bin/bash

SCRIPT_PATH="$( cd "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"
BUILD_PATH="$SCRIPT_PATH/build"

if [ ! -d $BUILD_PATH ]; then
	echo "$BUILD_PATH doesn't exist. Run \"make build-bls\" first." >/dev/stderr
	exit 1
fi

# Install the library
cmake -P $BUILD_PATH/cmake_install.cmake

exit 0
