#! /bin/bash
set -e

pwd

# run the go unit tests with coverage
bash test/test_cover.sh

# run the app tests using bash
bash test/app/test.sh

# run the persistence tests using bash
bash test/persist/test.sh

# checkout every github.com/tendermint dir and run its tests
# NOTE: on release-* or master branches only
bash test/test_libs.sh
