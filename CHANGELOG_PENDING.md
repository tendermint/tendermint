# Unreleased Changes

Friendly reminder: We have a [bug bounty program](https://hackerone.com/cosmos).

## vX.X

Month, DD, YYYY

Special thanks to external contributors on this release:

### BREAKING CHANGES

- CLI/RPC/Config

  - [rpc] Remove the deprecated gRPC interface to the RPC service. (@creachadair)
  - [blocksync] \#7159 Remove support for disabling blocksync in any circumstance. (@tychoish)
  - [mempool] \#7171 Remove legacy mempool implementation. (@tychoish)

- Apps

  - [proto/tendermint] \#6976 Remove core protobuf files in favor of only housing them in the [tendermint/spec](https://github.com/tendermint/spec) repository.

- P2P Protocol

  - [p2p] \#7035 Remove legacy P2P routing implementation and associated configuration options. (@tychoish)
  - [p2p] \#7265 Peer manager reduces peer score for each failed dial attempts for peers that have not successfully dialed. (@tychoish)

- Go API

  - [libs/sync] \#7450 Internalize and remove the library. (@creachadair)
  - [libs/async] \#7449 Move library to internal. (@creachadair)
  - [pubsub] \#7231 Remove unbuffered subscriptions and rework the Subscription interface. (@creachadair)
  - [eventbus] \#7231 Move the EventBus type to the internal/eventbus package. (@creachadair)
  - [blocksync] \#7046 Remove v2 implementation of the blocksync service and recactor, which was disabled in the previous release. (@tychoish)
  - [p2p] \#7064 Remove WDRR queue implementation. (@tychoish)
  - [config] \#7169 `WriteConfigFile` now returns an error. (@tychoish)
  - [libs/service] \#7288 Remove SetLogger method on `service.Service` interface. (@tychoish)


- Blockchain Protocol

### FEATURES

- [rpc] [\#7270](https://github.com/tendermint/tendermint/pull/7270) Add `header` and `header_by_hash` RPC Client queries. (@fedekunze)
- [cli] [#7033](https://github.com/tendermint/tendermint/pull/7033) Add a `rollback` command to rollback to the previous tendermint state in the event of non-determinstic app hash or reverting an upgrade.
- [mempool, rpc] \#7041  Add removeTx operation to the RPC layer. (@tychoish)

### IMPROVEMENTS
- [internal/protoio] \#7325 Optimized `MarshalDelimited` by inlining the common case and using a `sync.Pool` in the worst case. (@odeke-em)

- [pubsub] \#7319 Performance improvements for the event query API (@creachadair)

### BUG FIXES

- fix: assignment copies lock value in `BitArray.UnmarshalJSON()` (@lklimek)
