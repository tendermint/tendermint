.PHONEY: all docs test install get_vendor_deps ensure_tools

GOTOOLS = \
	github.com/Masterminds/glide

STRING := ../../clipperhouse/stringer

all: test install

docs:
	@go get github.com/davecheney/godoc2md
	godoc2md $(REPO) > README.md

all: install test

install: 
	go install github.com/tendermint/go-wire/cmd/...

test:
	go test `glide novendor`

get_vendor_deps: ensure_tools
	@rm -rf vendor/
	@echo "--> Running glide install"
	@glide install

ensure_tools:
	go get $(GOTOOLS)

pigeon:
	pigeon -o expr/expr.go expr/expr.peg

tools:
	go get github.com/clipperhouse/gen
	@cd ${STRING} && git remote add haus https://github.com/hausdorff/stringer.git
	@cd ${STRING} && git fetch haus && git checkout fix-imports
	@cd ${STRING} && go install .
	@go install github.com/clipperhouse/gen

