FROM nixos/nix:latest

RUN apk add --no-cache build-base curl bash git ca-certificates go

RUN nix-channel --add https://nixos.org/channels/nixpkgs-unstable nixpkgs
RUN nix-channel --update

ENV GOPATH /go
ENV GOBIN $GOPATH/bin
ENV PATH $GOBIN:$PATH
ENV WORK /tendermint

RUN mkdir -p $WORK && mkdir -p $GOPATH/src/github.com/tendermint && ln -sf $WORK $GOPATH/src/github.com/tendermint/tendermint

ADD . $WORK

RUN cd $GOPATH/src/github.com/tendermint/tendermint && make get_dep2nix && make build_nix
