.PHONY: get_deps build all list_deps install

all: install

TMROOT = $${TMROOT:-$$HOME/.tendermint}

install: 
	go install github.com/tendermint/tendermint/cmd/tendermint

build: 
	go build -o build/tendermint github.com/tendermint/tendermint/cmd/tendermint

build_race: 
	go build -race -o build/tendermint github.com/tendermint/tendermint/cmd/tendermint

test: build
	-rm -rf ~/.tendermint_test_bak
	-mv ~/.tendermint_test ~/.tendermint_test_bak && true
	go test github.com/tendermint/tendermint/...

draw_deps:
	# requires brew install graphviz
	go get github.com/hirokidaichi/goviz
	goviz -i github.com/tendermint/tendermint/cmd/tendermint | dot -Tpng -o huge.png

list_deps:
	go list -f '{{join .Deps "\n"}}' github.com/tendermint/tendermint/... |  xargs go list -f '{{if not .Standard}}{{.ImportPath}}{{end}}'

get_deps:
	go get github.com/tendermint/tendermint/...

revision:
	-echo `git rev-parse --verify HEAD` > $(TMROOT)/revision
	-echo `git rev-parse --verify HEAD` >> $(TMROOT)/revision_history
