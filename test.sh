#!/usr/bin/env bash

set -e
echo "" > coverage.txt

echo "==> Running unit tests"
for d in $(go list ./... | grep -v vendor); do
    go test -race -coverprofile=profile.out -covermode=atomic "$d"
    if [ -f profile.out ]; then
        cat profile.out >> coverage.txt
        rm profile.out
    fi
done

echo "==> Running integration tests (./tests)"
find . -path ./vendor -prune -o -name "*.sock" -exec rm {} \;
# tests/test.sh requires that we run the installed cmds, must not be out of date
make install
bash tests/test.sh
