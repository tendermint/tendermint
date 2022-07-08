# Unreleased Changes

Friendly reminder: We have a [bug bounty program](https://hackerone.com/cosmos).

## vX.X

Month, DD, YYYY

Special thanks to external contributors on this release:

### BREAKING CHANGES

- CLI/RPC/Config

  - [rpc] \#7121 Remove the deprecated gRPC interface to the RPC service. (@creachadair)
  - [blocksync] \#7159 Remove support for disabling blocksync in any circumstance. (@tychoish)
  - [mempool] \#7171 Remove legacy mempool implementation. (@tychoish)
  - [rpc] \#7575 Rework how RPC responses are written back via HTTP. (@creachadair)
  - [rpc] \#7713 Remove unused options for websocket clients. (@creachadair)
  - [config] \#7930 Add new event subscription options and defaults. (@creachadair)
  - [rpc] \#7982 Add new Events interface and deprecate Subscribe. (@creachadair)
  - [cli] \#8081 make the reset command safe to use by intoducing `reset-state` command. Fixed by \#8259. (@marbar3778, @cmwaters)
  - [config] \#8222 default indexer configuration to null. (@creachadair)
  - [rpc] \#8570 rework timeouts to be per-method instead of global. (@creachadair)
  - [rpc] \#8624 deprecate `broadcast_tx_commit` and `braodcast_tx_sync` and `broadcast_tx_async` in favor of `braodcast_tx`. (@tychoish)
  - [config] \#8654 remove deprecated `seeds` field from config. Users should switch to `bootstrap-peers` instead. (@cmwaters)

- Apps

  - [tendermint/spec] \#7804 Migrate spec from [spec repo](https://github.com/tendermint/spec).
  - [abci] \#7984 Remove the locks preventing concurrent use of ABCI applications by Tendermint. (@tychoish)
  - [abci] \#8605 Remove info, log, events, gasUsed and mempoolError fields from ResponseCheckTx as they are not used by Tendermint. (@jmalicevic)
  - [abci] \#8664 Move `app_hash` parameter from `Commit` to `FinalizeBlock`. (@sergio-mena)
  - [abci] \#8656 Added cli command for `PrepareProposal`. (@jmalicevic)
  - [sink/psql] \#8637 tx_results emitted from psql sink are now json encoded, previously they were protobuf encoded
  - [abci] \#8901 Added cli command for `ProcessProposal`. (@hvanz)

- P2P Protocol

  - [p2p] \#7035 Remove legacy P2P routing implementation and associated configuration options. (@tychoish)
  - [p2p] \#7265 Peer manager reduces peer score for each failed dial attempts for peers that have not successfully dialed. (@tychoish)
  - [p2p] [\#7594](https://github.com/tendermint/tendermint/pull/7594) always advertise self, to enable mutual address discovery. (@altergui)
  - [p2p] \#8737 Introduce "inactive" peer label to avoid re-dialing incompatible peers. (@tychoish)
  - [p2p] \#8737 Increase frequency of dialing attempts to reduce latency for peer acquisition. (@tychoish)
  - [p2p] \#8737 Improvements to peer scoring and sorting to gossip a greater variety of peers during PEX. (@tychoish)
  - [p2p] \#8737 Track incoming and outgoing peers separately to ensure more peer slots open for incoming connections. (@tychoish)

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
  - [crypto] \#8412 \#8432 Remove `crypto/tmhash` package in favor of  small functions in `crypto` package and cleanup of unused functions. (@tychoish)

- Blockchain Protocol

### FEATURES

- [rpc] [\#7270](https://github.com/tendermint/tendermint/pull/7270) Add `header` and `header_by_hash` RPC Client queries. (@fedekunze)
- [rpc] [\#7701] Add `ApplicationInfo` to `status` rpc call which contains the application version. (@jonasbostoen)
- [cli] [#7033](https://github.com/tendermint/tendermint/pull/7033) Add a `rollback` command to rollback to the previous tendermint state in the event of non-determinstic app hash or reverting an upgrade.
- [mempool, rpc] \#7041  Add removeTx operation to the RPC layer. (@tychoish)
- [consensus] \#7354 add a new `synchrony` field to the `ConsensusParams` struct for controlling the parameters of the proposer-based timestamp algorithm. (@williambanfield)
- [consensus] \#7376 Update the proposal logic per the Propose-based timestamps specification so that the proposer will wait for the previous block time to occur before proposing the next block. (@williambanfield)
- [consensus] \#7391 Use the proposed block timestamp as the proposal timestamp. Update the block validation logic to ensure that the proposed block's timestamp matches the timestamp in the proposal message. (@williambanfield)
- [consensus] \#7415 Update proposal validation logic to Prevote nil if a proposal does not meet the conditions for Timelyness per the proposer-based timestamp specification. (@anca)
- [consensus] \#7382 Update block validation to no longer require the block timestamp to be the median of the timestamps of the previous commit. (@anca)
- [consensus] \#7711 Use the proposer timestamp for the first height instead of the genesis time. Chains will still start consensus at the genesis time. (@anca)
- [cli] \#8281 Add a tool to update old config files to the latest version. (@creachadair)
- [consenus] \#8514 move `RecheckTx` from the local node mempool config to a global `ConsensusParams` field in `BlockParams` (@cmwaters)
- [abci] ABCI++ [specified](https://github.com/tendermint/tendermint/tree/master/spec/abci%2B%2B). (@sergio-mena, @cmwaters, @josef-widder)
- [abci] ABCI++ [implemented](https://github.com/orgs/tendermint/projects/9). (@williambanfield, @thanethomson, @sergio-mena)

### IMPROVEMENTS

- [internal/protoio] \#7325 Optimized `MarshalDelimited` by inlining the common case and using a `sync.Pool` in the worst case. (@odeke-em)
- [consensus] \#6969 remove logic to 'unlock' a locked block.
- [evidence] \#7700 Evidence messages contain single Evidence instead of EvidenceList (@jmalicevic)
- [evidence] \#7802 Evidence pool emits events when evidence is validated and updates a metric when the number of evidence in the evidence pool changes. (@jmalicevic)
- [pubsub] \#7319 Performance improvements for the event query API (@creachadair)
- [node] \#7521 Define concrete type for seed node implementation (@spacech1mp)
- [rpc] \#7612 paginate mempool /unconfirmed_txs rpc endpoint (@spacech1mp)
- [light] [\#7536](https://github.com/tendermint/tendermint/pull/7536) rpc /status call returns info about the light client (@jmalicevic)
- [types] \#7765 Replace EvidenceData with EvidenceList to avoid unnecessary nesting of evidence fields within a block. (@jmalicevic)

### BUG FIXES

- fix: assignment copies lock value in `BitArray.UnmarshalJSON()` (@lklimek)
- [light] \#7640 Light Client: fix absence proof verification (@ashcherbakov)
- [light] \#7641 Light Client: fix querying against the latest height (@ashcherbakov)
- [cli] [#7837](https://github.com/tendermint/tendermint/pull/7837) fix app hash in state rollback. (@yihuang)
- [cli] \#8276 scmigrate: ensure target key is correctly renamed. (@creachadair)
- [cli] \#8294 keymigrate: ensure block hash keys are correctly translated. (@creachadair)
- [cli] \#8352 keymigrate: ensure transaction hash keys are correctly translated. (@creachadair)
- (indexer) \#8625 Fix overriding tx index of duplicated txs.
