# netmon
blockchain network monitor


#Quick Start

To get started, [install golang](https://golang.org/doc/install) and [set your $GOPATH](https://github.com/tendermint/tendermint/wiki/Setting-GOPATH).

Install `tendermint`, `tmsp`, and the `netmon`:

```
go get github.com/tendermint/tendermint/cmd/tendermint
go get github.com/tendermint/tmsp/cmd/...
go get github.com/tendermint/netmon
```

Initialize and start a local tendermint node with

```
tendermint init
dummy &
tendermint node --fast_sync=false --log_level=debug
```

In another window, start the netmon with

```
netmon monitor $GOPATH/src/github.com/tendermint/netmon/local-chain.json
```

Then visit your browser at http://localhost:46670.

The chain's rpc can be found at http://localhost:46657.

# Notes

The netmon expects a config file with a list of chains/validators to get started. A default one for a local chain is provided as local-chain.json. `netmon config` can be used to create a config file for a chain deployed with `mintnet`.

The API is available as GET requests with URI encoded parameters, or as JSONRPC POST requests. The JSONRPC methods are also exposed over websocket.

# TODO

- log metrics for charts
- mintnet rpc commands
- chain size
- val set changes
- more efficient locking / refactor for a big select loop
