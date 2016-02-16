.PHONY: get_deps build all list_deps install

all: get_deps install test

TMROOT = $${TMROOT:-$$HOME/.tendermint}
define NEWLINE


endef

install: get_deps
	go install github.com/tendermint/tendermint/cmd/tendermint

build: 
	go build -o build/tendermint github.com/tendermint/tendermint/cmd/tendermint

build_race: 
	go build -race -o build/tendermint github.com/tendermint/tendermint/cmd/tendermint

test: build
	go test github.com/tendermint/tendermint/...

test_novendor: build
	go test $$(glide novendor)

draw_deps:
	# requires brew install graphviz
	go get github.com/hirokidaichi/goviz
	goviz -i github.com/tendermint/tendermint/cmd/tendermint | dot -Tpng -o huge.png

list_deps:
	go list -f '{{join .Deps "\n"}}' github.com/tendermint/tendermint/... |  sort | uniq | xargs go list -f '{{if not .Standard}}{{.ImportPath}}{{end}}'

get_deps:
	go get -d github.com/tendermint/tendermint/...
	go list -f '{{join .TestImports "\n"}}' github.com/tendermint/tendermint/... | sort | uniq | xargs go get

update_deps:
	go get -d -u github.com/tendermint/tendermint/...

revision:
	-echo `git rev-parse --verify HEAD` > $(TMROOT)/revision
	-echo `git rev-parse --verify HEAD` >> $(TMROOT)/revision_history
