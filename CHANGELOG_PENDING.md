# Unreleased Changes

Friendly reminder: We have a [bug bounty program](https://hackerone.com/cosmos).

## vX.X

Month, DD, YYYY

Special thanks to external contributors on this release:

### BREAKING CHANGES

- CLI/RPC/Config

  - [rpc] \#7575 Rework how RPC responses are written back via HTTP. (@creachadair)
  - [rpc] \#7121 Remove the deprecated gRPC interface to the RPC service. (@creachadair)
  - [blocksync] \#7159 Remove support for disabling blocksync in any circumstance. (@tychoish)
  - [mempool] \#7171 Remove legacy mempool implementation. (@tychoish)

- Apps

  - [proto/tendermint] \#6976 Remove core protobuf files in favor of only housing them in the [tendermint/spec](https://github.com/tendermint/spec) repository.

- P2P Protocol

  - [p2p] \#7035 Remove legacy P2P routing implementation and associated configuration options. (@tychoish)
  - [p2p] \#7265 Peer manager reduces peer score for each failed dial attempts for peers that have not successfully dialed. (@tychoish)
  - [p2p] [\#7594](https://github.com/tendermint/tendermint/pull/7594) always advertise self, to enable mutual address discovery. (@altergui)

- Go API

  - [rpc] \#7474 Remove the "URI" RPC client. (@creachadair)
  - [libs/pubsub] \#7451 Internalize the pubsub packages. (@creachadair)
  - [libs/sync] \#7450 Internalize and remove the library. (@creachadair)
  - [libs/async] \#7449 Move library to internal. (@creachadair)
  - [pubsub] \#7231 Remove unbuffered subscriptions and rework the Subscription interface. (@creachadair)
  - [eventbus] \#7231 Move the EventBus type to the internal/eventbus package. (@creachadair)
  - [blocksync] \#7046 Remove v2 implementation of the blocksync service and recactor, which was disabled in the previous release. (@tychoish)
  - [p2p] \#7064 Remove WDRR queue implementation. (@tychoish)
  - [config] \#7169 `WriteConfigFile` now returns an error. (@tychoish)
  - [libs/service] \#7288 Remove SetLogger method on `service.Service` interface. (@tychoish)
  - [abci/client] \#7607 Simplify client interface (removes most "async" methods). (@creachadair)
  - [libs/json] \#7673 Remove the libs/json (tmjson) library. (@creachadair)

- Blockchain Protocol

### FEATURES

- [rpc] [\#7270](https://github.com/tendermint/tendermint/pull/7270) Add `header` and `header_by_hash` RPC Client queries. (@fedekunze)
- [rpc] [\#7701] Add `ApplicationInfo` to `status` rpc call which contains the application version. (@jonasbostoen)
- [cli] [#7033](https://github.com/tendermint/tendermint/pull/7033) Add a `rollback` command to rollback to the previous tendermint state in the event of non-determinstic app hash or reverting an upgrade.
- [mempool, rpc] \#7041  Add removeTx operation to the RPC layer. (@tychoish)
- [consensus] \#7354 add a new `synchrony` field to the `ConsensusParameter` struct for controlling the parameters of the proposer-based timestamp algorithm. (@williambanfield)
- [consensus] \#7376 Update the proposal logic per the Propose-based timestamps specification so that the proposer will wait for the previous block time to occur before proposing the next block. (@williambanfield)
- [consensus] \#7391 Use the proposed block timestamp as the proposal timestamp. Update the block validation logic to ensure that the proposed block's timestamp matches the timestamp in the proposal message. (@williambanfield)
- [consensus] \#7415 Update proposal validation logic to Prevote nil if a proposal does not meet the conditions for Timelyness per the proposer-based timestamp specification. (@anca)
- [consensus] \#7382 Update block validation to no longer require the block timestamp to be the median of the timestamps of the previous commit. (@anca)

### IMPROVEMENTS
- [internal/protoio] \#7325 Optimized `MarshalDelimited` by inlining the common case and using a `sync.Pool` in the worst case. (@odeke-em)
- [consensus] \#6969 remove logic to 'unlock' a locked block.
- [pubsub] \#7319 Performance improvements for the event query API (@creachadair)
- [node] \#7521 Define concrete type for seed node implementation (@spacech1mp)
- [rpc] \#7612 paginate mempool /unconfirmed_txs rpc endpoint (@spacech1mp)
- [light] [\#7536](https://github.com/tendermint/tendermint/pull/7536) rpc /status call returns info about the light client (@jmalicevic)

### BUG FIXES

- fix: assignment copies lock value in `BitArray.UnmarshalJSON()` (@lklimek)
