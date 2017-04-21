FROM golang:latest

RUN curl https://glide.sh/get | sh

RUN mkdir -p /go/src/github.com/tendermint/go-p2p
WORKDIR /go/src/github.com/tendermint/go-p2p

COPY glide.yaml /go/src/github.com/tendermint/go-p2p/
COPY glide.lock /go/src/github.com/tendermint/go-p2p/

RUN glide install

COPY . /go/src/github.com/tendermint/go-p2p
