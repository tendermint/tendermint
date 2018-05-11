FROM nixos/nix:latest

RUN nix-channel --add https://nixos.org/channels/nixpkgs-unstable nixpkgs
RUN nix-channel --update

RUN apk add --no-cache build-base curl bash

ENV WORK /tendermint

ADD . $WORK

RUN cd $WORK && make build_nix
