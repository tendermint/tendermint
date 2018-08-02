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

# Now copy in the code
# NOTE: this will overwrite whatever is in vendor/
COPY . $REPO

# install ABCI CLI
RUN make install_abci

# install Tendermint
RUN make install

# expose the volume for debugging
VOLUME $REPO

EXPOSE 26656
EXPOSE 26657
