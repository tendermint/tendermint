.PHONY: get_deps build all list_deps install

all: install

install: 
	go install github.com/tendermint/tendermint/cmd/tendermint
	go install github.com/tendermint/tendermint/cmd/barak
	go install github.com/tendermint/tendermint/cmd/debora
	go install github.com/tendermint/tendermint/cmd/filelogger

build: 
	go build -o build/tendermint github.com/tendermint/tendermint/cmd/tendermint
	go build -o build/barak github.com/tendermint/tendermint/cmd/barak
	go build -o build/debora github.com/tendermint/tendermint/cmd/debora
	go build -o build/filelogger github.com/tendermint/tendermint/cmd/filelogger

build_race: 
	go build -race -o build/tendermint github.com/tendermint/tendermint/cmd/tendermint
	go build -race -o build/barak github.com/tendermint/tendermint/cmd/barak
	go build -race -o build/debora github.com/tendermint/tendermint/cmd/debora
	go build -race -o build/filelogger github.com/tendermint/tendermint/cmd/filelogger

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

tendermint_root/priv_validator.json: tendermint_root/priv_validator.json.orig
	cp $< $@

economy: tendermint_root/priv_validator.json
	docker run -v $(CURDIR)/tendermint_root:/tendermint_root -p 46656:46656 tendermint

clean:
	rm -f tendermint tendermint_root/priv_validator.json
