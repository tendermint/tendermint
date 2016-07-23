# Pull base image.
FROM golang:1.6

ENV TENDERMINT_ORG $GOPATH/src/github.com/tendermint/
RUN mkdir -p $TENDERMINT_ORG
COPY . $TENDERMINT_ORG/tendermint
WORKDIR $TENDERMINT_ORG/tendermint
