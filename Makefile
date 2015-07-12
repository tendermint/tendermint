.PHONY: get_deps build all list_deps install

all: install

install: 
	go install github.com/tendermint/tendermint/cmd/tendermint
	go install github.com/tendermint/tendermint/cmd/barak
	go install github.com/tendermint/tendermint/cmd/debora
	go install github.com/tendermint/tendermint/cmd/stdinwriter
	go install github.com/tendermint/tendermint/cmd/logjack
	echo -n `git rev-parse --verify HEAD` > .revision

build: 
	go build -o build/tendermint github.com/tendermint/tendermint/cmd/tendermint
	go build -o build/barak github.com/tendermint/tendermint/cmd/barak
	go build -o build/debora github.com/tendermint/tendermint/cmd/debora
	go build -o build/stdinwriter github.com/tendermint/tendermint/cmd/stdinwriter
	go build -o build/logjack github.com/tendermint/tendermint/cmd/logjack

build_race: 
	go build -race -o build/tendermint github.com/tendermint/tendermint/cmd/tendermint
	go build -race -o build/barak github.com/tendermint/tendermint/cmd/barak
	go build -race -o build/debora github.com/tendermint/tendermint/cmd/debora
	go build -race -o build/stdinwriter github.com/tendermint/tendermint/cmd/stdinwriter
	go build -race -o build/logjack github.com/tendermint/tendermint/cmd/logjack

test: build
	go test github.com/tendermint/tendermint/...

draw_deps:
	# requires brew install graphviz
	go get github.com/hirokidaichi/goviz
	goviz -i github.com/tendermint/tendermint/cmd/tendermint | dot -Tpng -o huge.png

list_deps:
	go list -f '{{join .Deps "\n"}}' github.com/tendermint/tendermint/... |  xargs go list -f '{{if not .Standard}}{{.ImportPath}}{{end}}'

get_deps:
	go get github.com/tendermint/tendermint/...

gen_client:
	go get -u github.com/ebuchman/go-rpc-gen
	go install github.com/ebuchman/go-rpc-gen
	go generate rpc/core_client/*.go

revision:
	echo -n `git rev-parse --verify HEAD` > .revision

tendermint_root/priv_validator.json: tendermint_root/priv_validator.json.orig
	cp $< $@

clean:
	rm -f tendermint tendermint_root/priv_validator.json
