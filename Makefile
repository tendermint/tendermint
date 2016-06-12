.PHONY: get_deps build all list_deps install

all: get_deps install test

TMROOT = $${TMROOT:-$$HOME/.tendermint}
define NEWLINE


endef
NOVENDOR = go list github.com/tendermint/tendermint/... | grep -v /vendor/

install: get_deps
	go install github.com/tendermint/tendermint/cmd/tendermint

build: 
	go build -o build/tendermint github.com/tendermint/tendermint/cmd/tendermint

build_race: 
	go build -race -o build/tendermint github.com/tendermint/tendermint/cmd/tendermint

test: build
	go test `${NOVENDOR}`

test100: build
	for i in {1..100}; do make test; done

draw_deps:
	# requires brew install graphviz
	go get github.com/hirokidaichi/goviz
	goviz -i github.com/tendermint/tendermint/cmd/tendermint | dot -Tpng -o huge.png

list_deps:
	go list -f '{{join .Deps "\n"}}' github.com/tendermint/tendermint/... | \
		grep -v /vendor/ | sort | uniq | \
	  xargs go list -f '{{if not .Standard}}{{.ImportPath}}{{end}}'

get_deps:
	go get -d `${NOVENDOR}`
	go list -f '{{join .TestImports "\n"}}' github.com/tendermint/tendermint/... | \
		grep -v /vendor/ | sort | uniq | \
		xargs go get

update_deps:
	go get -d -u github.com/tendermint/tendermint/...

revision:
	-echo `git rev-parse --verify HEAD` > $(TMROOT)/revision
	-echo `git rev-parse --verify HEAD` >> $(TMROOT)/revision_history
