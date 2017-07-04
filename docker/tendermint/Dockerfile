FROM busybox
#Use --build-arg to change where the tendermint binary resides
ARG TENDERMINT_BINARY=tendermint
ENV TMHOME /tendermint
COPY $TENDERMINT_BINARY /usr/bin/tendermint
RUN adduser -h $TMHOME -D tendermint
VOLUME [ $TMHOME ]
EXPOSE 46656 46657
USER tendermint
ENTRYPOINT ["/usr/bin/tendermint"]
CMD ["node"]
WORKDIR $TMHOME
STOPSIGNAL SIGTERM

