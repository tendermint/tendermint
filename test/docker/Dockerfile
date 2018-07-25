FROM golang:1.10

# Add testing deps for curl
RUN echo 'deb http://httpredir.debian.org/debian testing main non-free contrib' >> /etc/apt/sources.list

# Grab deps (jq, hexdump, xxd, killall)
RUN apt-get update && \
  apt-get install -y --no-install-recommends \
  jq bsdmainutils vim-common psmisc netcat curl

# Setup tendermint repo
ENV REPO $GOPATH/src/github.com/tendermint/tendermint
ENV GOBIN $GOPATH/bin
WORKDIR $REPO

# Copy in the code
COPY . $REPO

# Install the vendored dependencies
# docker caching prevents reinstall on code change!
RUN make get_tools
RUN make get_vendor_deps

RUN go install ./cmd/tendermint
RUN go install ./abci/cmd/abci-cli

# expose the volume for debugging
VOLUME $REPO

EXPOSE 26656
EXPOSE 26657
