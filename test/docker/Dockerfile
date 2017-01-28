# Pull base image.
FROM golang:1.7.4

# Grab deps (jq, hexdump, xxd, killall)
RUN apt-get update && \
  apt-get install -y --no-install-recommends \
  jq bsdmainutils vim-common psmisc 

# Setup tendermint repo with vendored dependencies
# but without code - docker caching prevents reinstall on code change!
ENV REPO $GOPATH/src/github.com/tendermint/tendermint
WORKDIR $REPO
ADD glide.yaml glide.yaml
ADD glide.lock glide.lock
ADD Makefile Makefile
RUN make get_vendor_deps

# Now copy in the code
COPY . $REPO

RUN go install ./cmd/tendermint
RUN bash scripts/install_abci_apps.sh

# expose the volume for debugging
VOLUME $REPO

EXPOSE 46656
EXPOSE 46657
