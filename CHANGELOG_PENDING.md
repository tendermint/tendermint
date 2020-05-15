## v0.33.5


\*\*

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

- CLI/RPC/Config

- Apps

- P2P Protocol

- Go API

  - [privval] [\#4744](https://github.com/tendermint/tendermint/pull/4744) Remove deprecated `OldFilePV` (@melekes)
  - [mempool] [\#4759](https://github.com/tendermint/tendermint/pull/4759) Modify `Mempool#InitWAL` to return an error (@melekes)
  - [node] [\#4832](https://github.com/tendermint/tendermint/pull/4832) `ConfigureRPC` returns an error (@melekes)
  - [rpc] [\#4836](https://github.com/tendermint/tendermint/pull/4836) Overhaul `lib` folder (@melekes)
    Move lib/ folder to jsonrpc/.
    Rename:
      rpc package -> jsonrpc package
      rpcclient package -> client package
      rpcserver package -> server package
      JSONRPCClient to Client
      JSONRPCRequestBatch to RequestBatch
      JSONRPCCaller to Caller
      StartHTTPServer to Serve
      StartHTTPAndTLSServer to ServeTLS
      NewURIClient to NewURI
      NewJSONRPCClient to New
      NewJSONRPCClientWithHTTPClient to NewWithHTTPClient
      NewWSClient to NewWS
    Unexpose ResponseWriterWrapper
    Remove unused http_params.go


- Blockchain Protocol

### FEATURES:

- [pex] [\#4439](https://github.com/tendermint/tendermint/pull/4439) Use highwayhash for pex buckets (@tau3)

### IMPROVEMENTS:

- [abci/server] [\#4719](https://github.com/tendermint/tendermint/pull/4719) Print panic & stack trace to STDERR if logger is not set (@melekes)
- [types] [\#4638](https://github.com/tendermint/tendermint/pull/4638) Implement `Header#ValidateBasic` (@alexanderbez)
- [buildsystem] [\#4378](https://github.com/tendermint/tendermint/pull/4738) Replace build_c and install_c with TENDERMINT_BUILD_OPTIONS parsing. The following options are available:
  - nostrip: don't strip debugging symbols nor DWARF tables.
  - cleveldb: use cleveldb as db backend instead of goleveldb.
  - race: pass -race to go build and enable data race detection.
- [mempool] [\#4759](https://github.com/tendermint/tendermint/pull/4759) Allow ReapX and CheckTx functions to run in parallel (@melekes)
- [rpc/core] [\#4844](https://github.com/tendermint/tendermint/pull/4844) Do not lock consensus state in `/validators`, `/consensus_params` and `/status` (@melekes)

### BUG FIXES:

- [blockchain/v2] [\#4761](https://github.com/tendermint/tendermint/pull/4761) Fix excessive CPU usage caused by spinning on closed channels (@erikgrinaker)
- [blockchain/v2] Respect `fast_sync` option (@erikgrinaker)
- [light] [\#4741](https://github.com/tendermint/tendermint/pull/4741) Correctly return  `ErrSignedHeaderNotFound` and `ErrValidatorSetNotFound` on corresponding RPC errors (@erikgrinaker)
- [rpc] \#4805 Attempt to handle panics during panic recovery (@erikgrinaker)
- [types] [\#4764](https://github.com/tendermint/tendermint/pull/4764) Return an error if voting power overflows in `VerifyCommitTrusting` (@melekes)
- [privval] [\#4812](https://github.com/tendermint/tendermint/pull/4812) Retry `GetPubKey/SignVote/SignProposal` a few times before returning an error (@melekes)
