FROM alpine:3.7
MAINTAINER Greg Szabo <greg@tendermint.com>

#Default home for tendermint. The node command will look for $TMHOME/config/genesis.json at initialization.
ENV TMHOME /tendermint
RUN apk update && \
    apk upgrade && \
    apk --no-cache add curl jq && \
    addgroup tmuser && \
    adduser -S -G tmuser tmuser -h "$TMHOME"

USER tmuser
VOLUME [ $TMHOME ]
WORKDIR $TMHOME
EXPOSE 46656 46657
ENTRYPOINT ["/usr/bin/tendermint"]
CMD ["node", "--moniker=`hostname`"]
STOPSIGNAL SIGTERM

ARG BINARY=tendermint
COPY $BINARY /usr/bin/tendermint

