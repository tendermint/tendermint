FROM golang:1.15-alpine AS builder

RUN apk update && \
    apk upgrade && \
    apk add bash git gmp-dev sudo cmake build-base

COPY . /go/src/github.com/dashevo/tenderdash

WORKDIR /go/src/github.com/dashevo/tenderdash

RUN make install-bls
RUN make build-linux

FROM alpine:3.9
LABEL maintainer="developers@dash.org"

# Tenderdash will be looking for the genesis file in /tenderdash/config/genesis.json
# (unless you change `genesis_file` in config.toml). You can put your config.toml and
# private validator file into /tenderdash/config.
#
# The /tenderdash/data dir is used by tenderdash to store state.
ENV TMHOME /tenderdash

# OS environment setup
# Set user right away for determinism, create directory for persistence and give our user ownership
# jq and curl used for extracting `pub_key` from private validator while
# deploying tenderdash with Kubernetes. It is nice to have bash so the users
# could execute bash commands.
RUN apk update && \
    apk upgrade && \
    apk --no-cache add curl jq bash gmp libstdc++ && \
    addgroup tmuser && \
    adduser -S -G tmuser tmuser -h "$TMHOME"

# Run the container with tmuser by default. (UID=100, GID=1000)
USER tmuser

WORKDIR $TMHOME

# p2p, rpc and prometheus port
EXPOSE 26656 26657 26660

STOPSIGNAL SIGTERM

COPY --from=builder /go/src/github.com/dashevo/tenderdash/build/tenderdash /usr/bin/tenderdash

# You can overwrite these before the first run to influence
# config.json and genesis.json. Additionally, you can override
# CMD to add parameters to `tenderdash node`.
ENV PROXY_APP=kvstore MONIKER=dockernode CHAIN_ID=dockerchain

COPY ./DOCKER/docker-entrypoint.sh /usr/local/bin/

ENTRYPOINT ["docker-entrypoint.sh"]
CMD ["node"]

# Expose the data directory as a volume since there's mutable state in there
VOLUME [ "$TMHOME" ]
