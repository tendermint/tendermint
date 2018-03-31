FROM alpine:3.7
MAINTAINER Greg Szabo <greg@tendermint.com>

#Default home for tendermint
ENV TMHOME /tendermint
RUN apk --no-cache update && \
    apk --no-cache upgrade && \
    apk add --no-cache bash curl jq openssl && \
    mkdir -p $TMHOME && \
    addgroup tmuser && \
    adduser -S -G tmuser tmuser && \
    chown -R tmuser:tmuser $TMHOME

VOLUME [ $TMHOME ]
EXPOSE 46656 46657
ENTRYPOINT ["/usr/bin/tendermint"]
CMD ["node", "--moniker=`hostname`"]
WORKDIR $TMHOME
STOPSIGNAL SIGTERM

ARG BINARY=tendermint
COPY $BINARY /usr/bin/tendermint

