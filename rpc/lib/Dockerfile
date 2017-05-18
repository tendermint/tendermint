FROM golang:latest

RUN mkdir -p /go/src/github.com/tendermint/tendermint/rpc/lib
WORKDIR /go/src/github.com/tendermint/tendermint/rpc/lib

COPY Makefile /go/src/github.com/tendermint/tendermint/rpc/lib/
# COPY glide.yaml /go/src/github.com/tendermint/tendermint/rpc/lib/
# COPY glide.lock /go/src/github.com/tendermint/tendermint/rpc/lib/

COPY . /go/src/github.com/tendermint/tendermint/rpc/lib

RUN make get_deps
