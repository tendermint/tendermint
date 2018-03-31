FROM ubuntu:16.04
MAINTAINER Greg Szabo <greg@tendermint.com>

#Default home for tendermint. The node command will look for $TMHOME/config/genesis.json at initialization.
ENV TMHOME /tendermint
RUN apt-get -y update && \
    apt-get -y upgrade && \
    apt-get -y install curl jq && \
    adduser --system --home "$TMHOME" --group tmuser

USER tmuser
VOLUME [ $TMHOME ]
WORKDIR $TMHOME
EXPOSE 46656 46657
ENTRYPOINT ["/usr/bin/tendermint"]
CMD ["node", "--moniker=`hostname`"]
STOPSIGNAL SIGTERM

ARG BINARY=tendermint
COPY $BINARY /usr/bin/tendermint

