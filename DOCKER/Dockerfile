FROM alpine:3.9
LABEL maintainer="hello@tendermint.com"

# Tendermint will be looking for the genesis file in /tendermint/config/genesis.json
# (unless you change `genesis_file` in config.toml). You can put your config.toml and
# private validator file into /tendermint/config.
#
# The /tendermint/data dir is used by tendermint to store state.
ENV TMHOME /tendermint

# OS environment setup
# Set user right away for determinism, create directory for persistence and give our user ownership
# jq and curl used for extracting `pub_key` from private validator while
# deploying tendermint with Kubernetes. It is nice to have bash so the users
# could execute bash commands.
RUN apk update && \
    apk upgrade && \
    apk --no-cache add curl jq bash && \
    addgroup tmuser && \
    adduser -S -G tmuser tmuser -h "$TMHOME"

# Run the container with tmuser by default. (UID=100, GID=1000)
USER tmuser

WORKDIR $TMHOME

# p2p and rpc port
EXPOSE 26656 26657 26660

ENTRYPOINT ["/usr/bin/tendermint"]
CMD ["node"]
STOPSIGNAL SIGTERM

ARG BINARY=tendermint
COPY $BINARY /usr/bin/tendermint

# Create default configuration for docker run.
RUN /usr/bin/tendermint init && \
    sed -i \
      -e 's/^proxy_app\s*=.*/proxy_app = "kvstore"/' \
      -e 's/^moniker\s*=.*/moniker = "dockernode"/' \
      -e 's/^addr_book_strict\s*=.*/addr_book_strict = false/' \
      -e 's/^timeout_commit\s*=.*/timeout_commit = "500ms"/' \
      -e 's/^index_all_tags\s*=.*/index_all_tags = true/' \
      -e 's/^prometheus\s*=.*/prometheus = true/' \
      $TMHOME/config/config.toml && \
    sed -i \
      -e 's/^\s*"chain_id":.*/  "chain_id": "dockerchain",/' \
      $TMHOME/config/genesis.json

# Expose the data directory as a volume since there's mutable state in there
VOLUME [ $TMHOME ]

