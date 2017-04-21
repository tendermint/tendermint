FROM golang:latest

RUN mkdir -p /go/src/github.com/tendermint/tendermint/rpc
WORKDIR /go/src/github.com/tendermint/tendermint/rpc

COPY Makefile /go/src/github.com/tendermint/tendermint/rpc/
# COPY glide.yaml /go/src/github.com/tendermint/tendermint/rpc/
# COPY glide.lock /go/src/github.com/tendermint/tendermint/rpc/

COPY . /go/src/github.com/tendermint/tendermint/rpc

RUN make get_deps
