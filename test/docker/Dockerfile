FROM golang:1.15

ENV REPO $GOPATH/src/github.com/tendermint/tendermint
ENV GOBIN $GOPATH/bin
WORKDIR $REPO

# Copy in the code
# TODO: rewrite to only copy Makefile & other files?
COPY . $REPO

# Grab deps (jq, hexdump, xxd, killall)
RUN apt-get update && \
  apt-get install -y --no-install-recommends \
  jq bsdmainutils vim-common psmisc netcat curl

# Install the vendored dependencies
# docker caching prevents reinstall on code change!
RUN make tools \
    && make install_abci \
    && make install \
    && chmod u+x tendermint-entry.sh

# expose the volume for debugging
VOLUME $REPO

EXPOSE 26656 26657

ENTRYPOINT ["tendermint-entry.sh"]
