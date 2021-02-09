FROM golang:1.15 as bin-installer

WORKDIR $GOPATH/src/github.com/tendermint/

COPY /tendermint .

RUN make install_abci # install abci client
RUN make install      # install tendermint

FROM bin-installer as tendermint-tester

COPY --from=tendermint $GOPATH/bin/tendermint $GOPATH/bin/
COPY --from=tendermint $GOPATH/bin/abci-cli $GOPATH/bin/
COPY --from=tendermint $REPO/tools/Makefile .
COPY --from=tendermint $REPO/tests .

RUN apt-get update && \
  apt-get install -y --no-install-recommends \
  jq bsdmainutils vim-common psmisc netcat curl

RUN make tools        # from /tools Makefile

# expose the volume for debugging
VOLUME .

EXPOSE 26656 26657

ENTRYPOINT ["/tests/docker/tendermint-entry.sh"]
