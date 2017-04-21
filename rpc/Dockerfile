FROM golang:latest

RUN mkdir -p /go/src/github.com/tendermint/go-rpc
WORKDIR /go/src/github.com/tendermint/go-rpc

COPY Makefile /go/src/github.com/tendermint/go-rpc/
# COPY glide.yaml /go/src/github.com/tendermint/go-rpc/
# COPY glide.lock /go/src/github.com/tendermint/go-rpc/

COPY . /go/src/github.com/tendermint/go-rpc

RUN make get_deps
