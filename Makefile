all: build

build:
	go build -o tendermint github.com/tendermint/tendermint/cmd

build_race:
	go build -race -o tendermint github.com/tendermint/tendermint/cmd

test:
	go test github.com/tendermint/tendermint/...

list_deps:
	go list -f '{{join .Deps "\n"}}' github.com/tendermint/tendermint/... |  xargs go list -f '{{if not .Standard}}{{.ImportPath}}{{end}}'

get_deps:
	go get github.com/tendermint/tendermint/...
