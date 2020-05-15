## v0.33.5


\*\*

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

- CLI/RPC/Config

- Apps

- P2P Protocol

- Go API

  - [crypto] [\#4721](https://github.com/tendermint/tendermint/pull/4721) Remove `SimpleHashFromMap()` and `SimpleProofsFromMap()` (@erikgrinaker)
  - [privval] [\#4744](https://github.com/tendermint/tendermint/pull/4744) Remove deprecated `OldFilePV` (@melekes)
  - [mempool] [\#4759](https://github.com/tendermint/tendermint/pull/4759) Modify `Mempool#InitWAL` to return an error (@melekes)
  - [types] \#4798 Simplify `VerifyCommitTrusting` func + remove extra validation (@melekes)
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

- [evidence] [\#4532](https://github.com/tendermint/tendermint/pull/4532) Handle evidence from light clients (@melekes)
- [lite2] [\#4532](https://github.com/tendermint/tendermint/pull/4532) Submit conflicting headers, if any, to a full node & all witnesses (@melekes)

### IMPROVEMENTS:

- [types] [\#4638](https://github.com/tendermint/tendermint/pull/4638) Implement `Header#ValidateBasic` (@alexanderbez)
- [abci/server] [\#4719](https://github.com/tendermint/tendermint/pull/4719) Print panic & stack trace to STDERR if logger is not set (@melekes)
- [mempool] [\#4759](https://github.com/tendermint/tendermint/pull/4759) Allow ReapX and CheckTx functions to run in parallel (@melekes)
- [p2p/conn] \#4795 Return err on `signChallenge()` instead of panic
- [rpc/core] [\#4844](https://github.com/tendermint/tendermint/pull/4844) Do not lock consensus state in `/validators`, `/consensus_params` and `/status` (@melekes)

### BUG FIXES:

- [blockchain/v2] [\#4761](https://github.com/tendermint/tendermint/pull/4761) Fix excessive CPU usage caused by spinning on closed channels (@erikgrinaker)
- [light] [\#4741](https://github.com/tendermint/tendermint/pull/4741) Correctly return  `ErrSignedHeaderNotFound` and `ErrValidatorSetNotFound` on corresponding RPC errors (@erikgrinaker)
- [rpc] \#4805 Attempt to handle panics during panic recovery (@erikgrinaker)
- [types] [\#4764](https://github.com/tendermint/tendermint/pull/4764) Return an error if voting power overflows in `VerifyCommitTrusting` (@melekes)
- [privval] [\#4812](https://github.com/tendermint/tendermint/pull/4812) Retry `GetPubKey/SignVote/SignProposal` a few times before returning an error (@melekes)
