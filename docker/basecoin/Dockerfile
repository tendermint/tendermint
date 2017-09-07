FROM busybox
#Use --build-arg to change where the basecoin binary resides
ARG BASECOIN_BINARY=basecoin
ENV BCHOME /basecoin
COPY $BASECOIN_BINARY /usr/bin/basecoin
RUN adduser -h $BCHOME -D basecoin
VOLUME [ $BCHOME ]
EXPOSE 46658
USER basecoin
ENTRYPOINT ["/usr/bin/basecoin"]
CMD ["start","--without-tendermint"]
WORKDIR $BCHOME
STOPSIGNAL SIGTERM

