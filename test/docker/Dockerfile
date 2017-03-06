# Pull base image.
FROM golang:1.7.4

# Add testing deps for curl
RUN echo 'deb http://httpredir.debian.org/debian testing main non-free contrib' >> /etc/apt/sources.list

# Grab deps (jq, hexdump, xxd, killall)
RUN apt-get update && \
  apt-get install -y --no-install-recommends \
  jq bsdmainutils vim-common psmisc netcat curl

# Setup tendermint repo 
ENV REPO $GOPATH/src/github.com/tendermint/tendermint
WORKDIR $REPO

# Install the vendored dependencies before copying code
# docker caching prevents reinstall on code change!
ADD glide.yaml glide.yaml
ADD glide.lock glide.lock
ADD Makefile Makefile
RUN make get_vendor_deps

# Install the apps
ADD scripts scripts
RUN bash scripts/install_abci_apps.sh

# Now copy in the code
COPY . $REPO

RUN go install ./cmd/tendermint

# expose the volume for debugging
VOLUME $REPO

EXPOSE 46656
EXPOSE 46657
