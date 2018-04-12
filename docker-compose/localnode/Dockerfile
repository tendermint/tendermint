FROM alpine:3.7
MAINTAINER Greg Szabo <greg@tendermint.com>

RUN apk update && \
    apk upgrade && \
    apk --no-cache add curl jq file && \
    addgroup tmuser && \
    adduser -S -G tmuser tmuser -h /tendermint

USER tmuser
VOLUME [ /tendermint ]
WORKDIR /tendermint
EXPOSE 46656 46657
ENTRYPOINT ["/usr/bin/wrapper.sh"]
CMD ["node", "--proxy_app dummy"]
STOPSIGNAL SIGTERM

COPY --chown=tmuser:tmuser wrapper.sh /usr/bin/wrapper.sh

