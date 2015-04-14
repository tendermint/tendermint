.PHONY: get_deps build all list_deps

all: build

build: get_deps
	go build -o build/tendermint github.com/tendermint/tendermint/cmd/tendermint
	go build -o build/barak github.com/tendermint/tendermint/cmd/barak
	go build -o build/debora github.com/tendermint/tendermint/cmd/debora

build_race: get_deps
	go build -race -o build/tendermint github.com/tendermint/tendermint/cmd/tendermint
	go build -race -o build/barak github.com/tendermint/tendermint/cmd/barak
	go build -race -o build/debora github.com/tendermint/tendermint/cmd/debora

test: build
	go test github.com/tendermint/tendermint/...

list_deps:
	go list -f '{{join .Deps "\n"}}' github.com/tendermint/tendermint/... |  xargs go list -f '{{if not .Standard}}{{.ImportPath}}{{end}}'

get_deps:
	go get github.com/tendermint/tendermint/...

tendermint_root/priv_validator.json: tendermint_root/priv_validator.json.orig
	cp $< $@

economy: tendermint_root/priv_validator.json
	docker run -v $(CURDIR)/tendermint_root:/tendermint_root -p 8080:8080 tendermint

clean:
	rm -f tendermint tendermint_root/priv_validator.json
