FROM golang:1.7.4

# This is the release of tendermint to pull in.
ENV TM_VERSION 0.8.0

# Tendermint will be looking for genesis file in /tendermint (unless you change
# `genesis_file` in config.toml). You can put your config.toml and private
# validator file into /tendermint.
#
# The /tendermint/data dir is used by tendermint to store state.
ENV DATA_ROOT /tendermint
ENV TMROOT $DATA_ROOT

# Set user right away for determinism
RUN groupadd -r tmuser && \
    useradd -r -s /bin/false -g tmuser tmuser

# Create directory for persistence and give our user ownership
RUN mkdir -p $DATA_ROOT && \
    chown -R tmuser:tmuser $DATA_ROOT

# TODO replace with downloading a binary (this will allow us to replace golang
# base container with alpine|jessie - 600MB vs 50MB)
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      git && \
    rm -rf /var/lib/apt/lists/*
RUN mkdir -p $GOPATH/src/github.com/tendermint/tendermint && \
    cd $GOPATH/src/github.com/tendermint/tendermint && \
    git clone https://github.com/tendermint/tendermint.git . && \
    git fetch && \
    git reset --hard v$TM_VERSION && \
    make install

# Expose the data directory as a volume since there's mutable state in there
VOLUME $DATA_ROOT

EXPOSE 46656
EXPOSE 46657

ENTRYPOINT ["tendermint"]

# By default you'll get the dummy app
CMD ["node", "--moniker=`hostname`", "--proxy_app=dummy"]
