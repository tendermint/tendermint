.PHONY: get_deps build all list_deps

all: build

build: get_deps
	go build -o tendermint github.com/tendermint/tendermint/cmd

build_race: get_deps
	go build -race -o tendermint github.com/tendermint/tendermint/cmd

test: build
	go test github.com/tendermint/tendermint/...

list_deps:
	go list -f '{{join .Deps "\n"}}' github.com/tendermint/tendermint/... |  xargs go list -f '{{if not .Standard}}{{.ImportPath}}{{end}}'

get_deps:
	go get github.com/tendermint/tendermint/...

clean:
	rm -f tendermint
