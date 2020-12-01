#!/bin/bash

SCRIPT_PATH="$( cd "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"
SRC_PATH="$SCRIPT_PATH/src"
BUILD_PATH="$SCRIPT_PATH/build"

git submodule update --init third_party/bls-signatures/src

# Create folders for source and build data
mkdir -p $BUILD_PATH

# Configurate the library build
cmake -B $BUILD_PATH -S $SRC_PATH

# Build the library
make -C $BUILD_PATH

exit 0
