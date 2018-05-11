FROM nixos/nix:latest

RUN nix-channel --add https://nixos.org/channels/nixpkgs-unstable nixpkgs
RUN nix-channel --update

RUN apk add --no-cache build-base curl bash git go

ENV WORK /tendermint
ENV GOPATH /go
ENV PATH $GOPATH/bin:$PATH

RUN mkdir -p $GOPATH/src && ln -sf $GOPATH/src/tendermint $WORK

ADD . $WORK

RUN cd $GOPATH/src/tendermint && make get_tools && make get_vendor_deps && make get_dep2nix && make build_nix
