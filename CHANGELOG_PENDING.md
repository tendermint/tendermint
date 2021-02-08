# Unreleased Changes

## vX.X

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES

- CLI/RPC/Config
  - [config] \#5598 The `test_fuzz` and `test_fuzz_config` P2P settings have been removed. (@erikgrinaker)
  - [config] \#5728 `fast_sync = "v1"` is no longer supported (@melekes)
  - [cli] \#5772 `gen_node_key` prints JSON-encoded `NodeKey` rather than ID and does not save it to `node_key.json` (@melekes)
  - [cli] \#5777 use hyphen-case instead of snake_case for all cli commands and config parameters (@cmwaters)

- Apps
  - [ABCI] \#5447 Remove `SetOption` method from `ABCI.Client` interface
  - [ABCI] \#5447 Reset `Oneof` indexes for  `Request` and `Response`.
  - [ABCI] \#5818 Use protoio for msg length delimitation. Migrates from int64 to uint64 length delimiters.

- P2P Protocol

- Go API
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
  - [rpc/client/http] \#6022 Change `timeout` type to `time.Duration` in `NewWithTimeout`
  - [light] \#6054 Move `MaxRetryAttempt` option from client to provider.
    - `NewWithOptions` now sets the max retry attempts and timeouts (@cmwaters)

- Blockchain Protocol

- Data Storage
  - [store/state/evidence/light] \#5771 Use an order-preserving varint key encoding (@cmwaters)

### FEATURES

### IMPROVEMENTS

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
- [consensus] \#5987 Remove `time_iota_ms` from consensus params. Merge `tmproto.ConsensusParams` and `abci.ConsensusParams`. (@marbar3778)
- [types] \#5994 Reduce the use of protobuf types in core logic. (@marbar3778)
  - `ConsensusParams`, `BlockParams`, `ValidatorParams`, `EvidenceParams`, `VersionParams`, `sm.Version` and `version.Consensus` have become native types. They still utilize protobuf when being sent over the wire or written to disk.

### BUG FIXES

- [types] \#5523 Change json naming of `PartSetHeader` within `BlockID` from `parts` to `part_set_header` (@marbar3778)
- [privval] \#5638 Increase read/write timeout to 5s and calculate ping interval based on it (@JoeKash)
- [blockchain/v1] [\#5701](https://github.com/tendermint/tendermint/pull/5701) Handle peers without blocks (@melekes)
- [blockchain/v1] \#5711 Fix deadlock (@melekes)
- [light] \#6022 Fix a bug when the number of validators equals 100 (@melekes)
- [light] \#6026 Fix a bug when height isn't provided for the rpc calls: `/commit` and `/validators` (@cmwaters)
