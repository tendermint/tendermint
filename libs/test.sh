#!/usr/bin/env bash
set -e

# run the linter
# make lint

# setup certs
make gen_certs

# run the unit tests with coverage
echo "" > coverage.txt
for d in $(go list ./... | grep -v vendor); do
	go test -race -coverprofile=profile.out -covermode=atomic "$d"
	if [ -f profile.out ]; then
		cat profile.out >> coverage.txt
		rm profile.out
	fi
done

# cleanup certs
make clean_certs
