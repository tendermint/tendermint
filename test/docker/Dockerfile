# Pull base image.
FROM golang:1.6

# Grab deps (jq, hexdump, xxd, killall)
RUN apt-get update && \
  apt-get install -y --no-install-recommends \
  jq bsdmainutils vim-common psmisc 

ENV REPO $GOPATH/src/github.com/tendermint/tendermint
WORKDIR $REPO
ADD glide.yaml glide.yaml
ADD glide.lock glide.lock
ADD Makefile Makefile
RUN make get_vendor_deps

COPY . $REPO

RUN go install ./cmd/tendermint
RUN bash scripts/install_tmsp_apps.sh

EXPOSE 46656
EXPOSE 46657
