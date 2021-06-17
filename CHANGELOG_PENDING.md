# Unreleased Changes

## vX.X

Special thanks to external contributors on this release:

Friendly reminder: We have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES

- CLI/RPC/Config
  - [config] \#5598 The `test_fuzz` and `test_fuzz_config` P2P settings have been removed. (@erikgrinaker)
  - [config] \#5728 `fast_sync = "v1"` is no longer supported (@melekes)
  - [cli] \#5772 `gen_node_key` prints JSON-encoded `NodeKey` rather than ID and does not save it to `node_key.json` (@melekes)
  - [cli] \#5777 use hyphen-case instead of snake_case for all cli commands and config parameters (@cmwaters)
  - [rpc] \#6019 standardise RPC errors and return the correct status code (@bipulprasad & @cmwaters)
  - [rpc] \#6168 Change default sorting to desc for `/tx_search` results (@melekes)
  - [cli] \#6282 User must specify the node mode when using `tendermint init` (@cmwaters)
  - [state/indexer] \#6382 reconstruct indexer, move txindex into the indexer package (@JayT106)
  - [cli] \#6372 Introduce `BootstrapPeers` as part of the new p2p stack. Peers to be connected on
    startup (@cmwaters)
  - [config] \#6462 Move `PrivValidator` configuration out of `BaseConfig` into its own section.

- Apps
  - [ABCI] \#6408 Change the `key` and `value` fields from `[]byte` to `string` in the `EventAttribute` type. (@alexanderbez)
  - [ABCI] \#5447 Remove `SetOption` method from `ABCI.Client` interface
  - [ABCI] \#5447 Reset `Oneof` indexes for  `Request` and `Response`.
  - [ABCI] \#5818 Use protoio for msg length delimitation. Migrates from int64 to uint64 length delimiters.
  - [Version] \#6494 `TMCoreSemVer` has been renamed to `TMVersion`.
    - It is not required any longer to set ldflags to set version strings

- P2P Protocol

- Go API
  - [logging] \#6534 Removed the existing custom Tendermint logger backed by go-kit. The logging interface, `Logger`, remains.
  Tendermint still provides a default logger backed by the performant zerolog logger. (@alexanderbez)
  - [mempool] \#6529 The `Context` field has been removed from the `TxInfo` type. `CheckTx` now requires a `Context` argument. (@alexanderbez)
  - [abci/client, proxy] \#5673 `Async` funcs return an error, `Sync` and `Async` funcs accept `context.Context` (@melekes)
  - [p2p] Removed unused function `MakePoWTarget`. (@erikgrinaker)
  - [libs/bits] \#5720 Validate `BitArray` in `FromProto`, which now returns an error (@melekes)
  - [proto/p2p] Renamed `DefaultNodeInfo` and `DefaultNodeInfoOther` to `NodeInfo` and `NodeInfoOther` (@erikgrinaker)
  - [proto/p2p] Rename `NodeInfo.default_node_id` to `node_id` (@erikgrinaker)
  - [libs/os] Kill() and {Must,}{Read,Write}File() functions have been removed. (@alessio)
  - [store] \#5848 Remove block store state in favor of using the db iterators directly (@cmwaters)
  - [state] \#5864 Use an iterator when pruning state (@cmwaters)
  - [types] \#6023 Remove `tm2pb.Header`, `tm2pb.BlockID`, `tm2pb.PartSetHeader` and `tm2pb.NewValidatorUpdate`.
    - Each of the above types has a `ToProto` and `FromProto` method or function which replaced this logic.
  - [light] \#6054 Move `MaxRetryAttempt` option from client to provider.
    - `NewWithOptions` now sets the max retry attempts and timeouts (@cmwaters)
  - [all] \#6077 Change spelling from British English to American (@cmwaters)
    - Rename "Subscription.Cancelled()" to "Subscription.Canceled()" in libs/pubsub
    - Rename "behaviour" pkg to "behavior" and internalized it in blockchain v2
  - [rpc/client/http] \#6176 Remove `endpoint` arg from `New`, `NewWithTimeout` and `NewWithClient` (@melekes)
  - [rpc/client/http] \#6176 Unexpose `WSEvents` (@melekes)
  - [rpc/jsonrpc/client/ws_client] \#6176 `NewWS` no longer accepts options (use `NewWSWithOptions` and `OnReconnect` funcs to configure the client) (@melekes)
  - [internal/libs] \#6366 Move `autofile`, `clist`,`fail`,`flowrate`, `protoio`, `sync`, `tempfile`, `test` and `timer` lib packages to an internal folder
  - [libs/rand] \#6364 Removed most of libs/rand in favour of standard lib's `math/rand` (@liamsi)
  - [mempool] \#6466 The original mempool reactor has been versioned as `v0` and moved to a sub-package under the root `mempool` package.
    Some core types have been kept in the `mempool` package such as `TxCache` and it's implementations, the `Mempool` interface itself
    and `TxInfo`. (@alexanderbez)

- Blockchain Protocol

- Data Storage
  - [store/state/evidence/light] \#5771 Use an order-preserving varint key encoding (@cmwaters)
  - [mempool] \#6396 Remove mempool's write ahead log (WAL), (previously unused by the tendermint code). (@tychoish)
  - [state] \#6541 Move pruneBlocks from consensus/state to state/execution. (@JayT106)

- Tooling
  - [tools] \#6498 Set OS home dir to instead of the hardcoded PATH. (@JayT106)

### FEATURES

- [config] Add `--mode` flag and config variable. See [ADR-52](https://github.com/tendermint/tendermint/blob/master/docs/architecture/adr-052-tendermint-mode.md) @dongsam
- [rpc] \#6329 Don't cap page size in unsafe mode (@gotjoshua, @cmwaters)
- [pex] \#6305 v2 pex reactor with backwards compatability. Introduces two new pex messages to
  accomodate for the new p2p stack. Removes the notion of seeds and crawling. All peer
  exchange reactors behave the same. (@cmwaters)
- [crypto] \#6376 Enable sr25519 as a validator key
- [mempool] \#6466 Introduction of a prioritized mempool. (@alexanderbez)
  - `Priority` and `Sender` have been introduced into the `ResponseCheckTx` type, where the `priority` will determine the prioritization of
  the transaction when a proposer reaps transactions for a block proposal. The `sender` field acts as an index.
  - Operators may toggle between the legacy mempool reactor, `v0`, and the new prioritized reactor, `v1`, by setting the
  `mempool.version` configuration, where `v1` is the default configuration.
  - Applications that do not specify a priority, i.e. zero, will have transactions reaped by the order in which they are received by the node.
  - Transactions are gossiped in FIFO order as they are in `v0`.
- [config/indexer] \#6411 Introduce support for custom event indexing data sources, specifically PostgreSQL. (@JayT106)

### IMPROVEMENTS

- [statesync] \#6566 Allow state sync fetchers and request timeout to be configurable. (@alexanderbez)
- [types] \#6478 Add `block_id` to `newblock` event (@jeebster)
- [crypto/ed25519] \#5632 Adopt zip215 `ed25519` verification. (@marbar3778)
- [privval] \#5603 Add `--key` to `init`, `gen_validator`, `testnet` & `unsafe_reset_priv_validator` for use in generating `secp256k1` keys.
- [privval] \#5725 Add gRPC support to private validator.
- [privval] \#5876 `tendermint show-validator` will query the remote signer if gRPC is being used (@marbar3778)
- [abci/client] \#5673 `Async` requests return an error if queue is full (@melekes)
- [mempool] \#5673 Cancel `CheckTx` requests if RPC client disconnects or times out (@melekes)
- [abci] \#5706 Added `AbciVersion` to `RequestInfo` allowing applications to check ABCI version when connecting to Tendermint. (@marbar3778)
- [blockchain/v1] \#5728 Remove in favor of v2 (@melekes)
- [blockchain/v0] \#5741 Relax termination conditions and increase sync timeout (@melekes)
- [cli] \#5772 `gen_node_key` output now contains node ID (`id` field) (@melekes)
- [blockchain/v2] \#5774 Send status request when new peer joins (@melekes)
- [consensus] \#5792 Deprecates the `time_iota_ms` consensus parameter, to reduce the bug surface. The parameter is no longer used. (@valardragon)
- [store] \#5888 store.SaveBlock saves using batches instead of transactions for now to improve ACID properties. This is a quick fix for underlying issues around tm-db and ACID guarantees. (@githubsands)
- [consensus] \#5987 Remove `time_iota_ms` from consensus params. Merge `tmproto.ConsensusParams` and `abci.ConsensusParams`. (@marbar3778)
- [types] \#5994 Reduce the use of protobuf types in core logic. (@marbar3778)
  - `ConsensusParams`, `BlockParams`, `ValidatorParams`, `EvidenceParams`, `VersionParams`, `sm.Version` and `version.Consensus` have become native types. They still utilize protobuf when being sent over the wire or written to disk.
- [rpc/client/http] \#6163 Do not drop events even if the `out` channel is full (@melekes)
- [node] \#6059 Validate and complete genesis doc before saving to state store (@silasdavis)
- [state] \#6067 Batch save state data (@githubsands & @cmwaters)
- [crypto] \#6120 Implement batch verification interface for ed25519 and sr25519. (@marbar3778)
- [types] \#6120 use batch verification for verifying commits signatures.
  - If the key type supports the batch verification API it will try to batch verify. If the verification fails we will single verify each signature.
- [privval/file] \#6185 Return error on `LoadFilePV`, `LoadFilePVEmptyState`. Allows for better programmatic control of Tendermint.
- [privval] \#6240 Add `context.Context` to privval interface.
- [rpc] \#6265 set cache control in http-rpc response header (@JayT106)
- [statesync] \#6378 Retry requests for snapshots and add a minimum discovery time (5s) for new snapshots.
- [node/state] \#6370 graceful shutdown in the consensus reactor (@JayT106)
- [crypto/merkle] \#6443 Improve HashAlternatives performance (@cuonglm)
- [crypto/merkle] \#6513 Optimize HashAlternatives (@marbar3778)
- [p2p/pex] \#6509 Improve addrBook.hash performance (@cuonglm)
- [consensus/metrics] \#6549 Change block_size gauge to a histogram for better observability over time (@marbar3778)

### BUG FIXES

- [privval] \#5638 Increase read/write timeout to 5s and calculate ping interval based on it (@JoeKash)
- [blockchain/v1] [\#5701](https://github.com/tendermint/tendermint/pull/5701) Handle peers without blocks (@melekes)
- [blockchain/v1] \#5711 Fix deadlock (@melekes)
- [evidence] \#6375 Fix bug with inconsistent LightClientAttackEvidence hashing (cmwaters)
- [rpc] \#6507 fix RPC client doesn't handle url's without ports (@JayT106)
- [statesync] \#6463 Adds Reverse Sync feature to fetch historical light blocks after state sync in order to verify any evidence (@cmwaters) 
