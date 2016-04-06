.PHONY: get_deps build all list_deps install

all: install

TMROOT = $${TMROOT:-$$HOME/.tendermint}

install: 
	go install github.com/eris-ltd/tendermint/cmd/tendermint
	go install github.com/eris-ltd/tendermint/cmd/barak
	go install github.com/eris-ltd/tendermint/cmd/debora
	go install github.com/eris-ltd/tendermint/cmd/stdinwriter
	go install github.com/eris-ltd/tendermint/cmd/logjack
	go install github.com/eris-ltd/tendermint/cmd/sim_txs

build: 
	go build -o build/tendermint github.com/eris-ltd/tendermint/cmd/tendermint
	go build -o build/barak github.com/eris-ltd/tendermint/cmd/barak
	go build -o build/debora github.com/eris-ltd/tendermint/cmd/debora
	go build -o build/stdinwriter github.com/eris-ltd/tendermint/cmd/stdinwriter
	go build -o build/logjack github.com/eris-ltd/tendermint/cmd/logjack
	go build -o build/sim_txs github.com/eris-ltd/tendermint/cmd/sim_txs

build_race: 
	go build -race -o build/tendermint github.com/eris-ltd/tendermint/cmd/tendermint
	go build -race -o build/barak github.com/eris-ltd/tendermint/cmd/barak
	go build -race -o build/debora github.com/eris-ltd/tendermint/cmd/debora
	go build -race -o build/stdinwriter github.com/eris-ltd/tendermint/cmd/stdinwriter
	go build -race -o build/logjack github.com/eris-ltd/tendermint/cmd/logjack
	go build -race -o build/sim_txs github.com/eris-ltd/tendermint/cmd/sim_txs

test: build
	-rm -rf ~/.tendermint_test_bak
	-mv ~/.tendermint_test ~/.tendermint_test_bak && true
	go test github.com/eris-ltd/tendermint/...

draw_deps:
	# requires brew install graphviz
	go get github.com/hirokidaichi/goviz
	goviz -i github.com/eris-ltd/tendermint/cmd/tendermint | dot -Tpng -o huge.png

list_deps:
	go list -f '{{join .Deps "\n"}}' github.com/eris-ltd/tendermint/... |  xargs go list -f '{{if not .Standard}}{{.ImportPath}}{{end}}'

get_deps:
	go get github.com/eris-ltd/tendermint/...

gen_client:
	go get -u github.com/ebuchman/go-rpc-gen
	go install github.com/ebuchman/go-rpc-gen
	go generate rpc/core_client/*.go

revision:
	-echo `git rev-parse --verify HEAD` > $(TMROOT)/revision
	-echo `git rev-parse --verify HEAD` >> $(TMROOT)/revision_history
