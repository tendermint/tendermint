FROM alpine:3.7

ENV DATA_ROOT /tendermint
ENV TMHOME $DATA_ROOT

RUN addgroup tmuser && \
    adduser -S -G tmuser tmuser

RUN mkdir -p $DATA_ROOT && \
    chown -R tmuser:tmuser $DATA_ROOT

RUN apk add --no-cache bash curl jq

ENV GOPATH /go
ENV PATH "$PATH:/go/bin"
RUN mkdir -p /go/src/github.com/tendermint/tendermint && \
    apk add --no-cache go build-base git && \
    cd /go/src/github.com/tendermint/tendermint && \
    git clone https://github.com/tendermint/tendermint . && \
    git checkout develop && \
    make get_tools && \
    make get_vendor_deps && \
    make install && \
    cd - && \
    rm -rf /go/src/github.com/tendermint/tendermint && \
    apk del go build-base git

VOLUME $DATA_ROOT

EXPOSE 26656
EXPOSE 26657

ENTRYPOINT ["tendermint"]

CMD ["node", "--moniker=`hostname`", "--proxy_app=kvstore"]
