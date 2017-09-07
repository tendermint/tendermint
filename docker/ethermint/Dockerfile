FROM busybox
#Use --build-arg to change where the ethermint binary and setup directory resides
ARG ETHERMINT_BINARY=ethermint
ARG SETUP_DIR=setup
ENV EMHOME /ethermint
COPY $ETHERMINT_BINARY /usr/bin/ethermint
COPY $SETUP_DIR /setup
RUN adduser -h $EMHOME -D ethermint
VOLUME [ $EMHOME ]
EXPOSE 46658
USER ethermint
ENTRYPOINT ["/usr/bin/ethermint","--datadir","$EMHOME"]
WORKDIR $EMHOME
STOPSIGNAL SIGTERM

