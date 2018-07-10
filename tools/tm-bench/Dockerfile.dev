FROM golang:latest

RUN mkdir -p /go/src/github.com/tendermint/tendermint/tools/tm-bench
WORKDIR /go/src/github.com/tendermint/tendermint/tools/tm-bench

COPY Makefile /go/src/github.com/tendermint/tendermint/tools/tm-bench/

RUN make get_tools

COPY . /go/src/github.com/tendermint/tendermint/tools/tm-bench

RUN make get_vendor_deps
