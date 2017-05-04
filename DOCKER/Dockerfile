FROM alpine:3.5

# This is the release of tendermint to pull in.
ENV TM_VERSION 0.9.1
ENV TM_SHA256SUM da34234755937140dcd953afcc965555fad7e05afd546711bc5bdc2df3d54226

# Tendermint will be looking for genesis file in /tendermint (unless you change
# `genesis_file` in config.toml). You can put your config.toml and private
# validator file into /tendermint.
#
# The /tendermint/data dir is used by tendermint to store state.
ENV DATA_ROOT /tendermint
ENV TMHOME $DATA_ROOT

# Set user right away for determinism
RUN addgroup tmuser && \
    adduser -S -G tmuser tmuser

# Create directory for persistence and give our user ownership
RUN mkdir -p $DATA_ROOT && \
    chown -R tmuser:tmuser $DATA_ROOT

# jq and curl used for extracting `pub_key` from private validator while
# deploying tendermint with Kubernetes. It is nice to have bash so the users
# could execute bash commands.
RUN apk add --no-cache bash curl jq

RUN apk add --no-cache openssl && \
    wget https://s3-us-west-2.amazonaws.com/tendermint/${TM_VERSION}/tendermint_${TM_VERSION}_linux_amd64.zip && \
    echo "${TM_SHA256SUM}  tendermint_${TM_VERSION}_linux_amd64.zip" | sha256sum -c && \
    unzip -d /bin tendermint_${TM_VERSION}_linux_amd64.zip && \
    apk del openssl && \
    rm -f tendermint_${TM_VERSION}_linux_amd64.zip

# Expose the data directory as a volume since there's mutable state in there
VOLUME $DATA_ROOT

# p2p port
EXPOSE 46656
# rpc port
EXPOSE 46657

ENTRYPOINT ["tendermint"]

# By default you'll get the dummy app
CMD ["node", "--moniker=`hostname`", "--proxy_app=dummy"]
