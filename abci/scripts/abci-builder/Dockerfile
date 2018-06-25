FROM golang:1.9.2

RUN apt-get update && apt-get install -y --no-install-recommends \
		zip \
	&& rm -rf /var/lib/apt/lists/*

# We want to ensure that release builds never have any cgo dependencies so we
# switch that off at the highest level.
ENV CGO_ENABLED 0

RUN mkdir -p $GOPATH/src/github.com/tendermint/abci
WORKDIR $GOPATH/src/github.com/tendermint/abci
