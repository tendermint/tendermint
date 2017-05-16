FROM golang:1.8
MAINTAINER adrian@brink-holdings.com

ARG priv_validator
ARG genesis

ENV DATA_ROOT /tendermint
ENV TMROOT $DATA_ROOT

RUN useradd -ms /bin/bash tmuser

RUN mkdir -p $DATA_ROOT && \
    chown -R tmuser:tmuser $DATA_ROOT

ADD $genesis $TMROOT/genesis.json
ADD $priv_validator $TMROOT/priv_validator.json

RUN go get github.com/Masterminds/glide
RUN go get -d github.com/tendermint/tendermint/cmd/tendermint
WORKDIR $GOPATH/src/github.com/tendermint/tendermint
RUN glide install
RUN go install ./cmd/tendermint

# VOLUME $DATA_ROOT

# p2p
#EXPOSE 46656

# rpc
#EXPOSE 46657

ENTRYPOINT ["tendermint"]