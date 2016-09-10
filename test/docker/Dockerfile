# Pull base image.
FROM golang:1.6

# Grab deps (jq, hexdump, xxd)
RUN apt-get update && \
  apt-get install -y --no-install-recommends \
  jq bsdmainutils vim-common && \ 
  rm -rf /var/lib/apt/lists/*

ENV TENDERMINT_ORG $GOPATH/src/github.com/tendermint/
RUN mkdir -p $TENDERMINT_ORG
COPY . $TENDERMINT_ORG/tendermint
WORKDIR $TENDERMINT_ORG/tendermint

RUN make get_vendor_deps


RUN go install ./cmd/tendermint
RUN bash scripts/install_tmsp_apps.sh

EXPOSE 46656
EXPOSE 46657
