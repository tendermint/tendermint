# Unreleased Changes

## vX.X

Special thanks to external contributors on this release:

@p4u from vocdoni.io reported that the mempool might behave incorrectly under a
high load. The consequences can range from pauses between blocks to the peers
disconnecting from this node. As a temporary remedy (until the mempool package
is refactored), the `max-batch-bytes` was disabled. Transactions will be sent
one by one without batching.

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES

- CLI/RPC/Config
  - [config] \#5598 The `test_fuzz` and `test_fuzz_config` P2P settings have been removed. (@erikgrinaker)
  - [config] \#5728 `fast_sync = "v1"` is no longer supported (@melekes)
  - [cli] \#5772 `gen_node_key` prints JSON-encoded `NodeKey` rather than ID and does not save it to `node_key.json` (@melekes)
  - [cli] \#5777 use hypen-case instead of snake_case for all cli comamnds and config parameters

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
  - [libs/os] `EnsureDir` now propagates IO errors and checks the file type (@erikgrinaker)
  - [libs/os] Kill() and {Must,}{Read,Write}File() functions have been removed. (@alessio)

- Blockchain Protocol

- Data Storage
  - [store/state/evidence/light] \#5771 Use an order-preserving varint key encoding (@cmwaters)

### FEATURES

### IMPROVEMENTS

- [crypto/ed25519] \#5632 Adopt zip215 `ed25519` verification. (@marbar3778)
- [privval] \#5603 Add `--key` to `init`, `gen_validator`, `testnet` & `unsafe_reset_priv_validator` for use in generating `secp256k1` keys.
- [abci/client] \#5673 `Async` requests return an error if queue is full (@melekes)
- [mempool] \#5673 Cancel `CheckTx` requests if RPC client disconnects or times out (@melekes)
- [abci] \#5706 Added `AbciVersion` to `RequestInfo` allowing applications to check ABCI version when connecting to Tendermint. (@marbar3778)
- [blockchain/v1] \#5728 Remove in favor of v2 (@melekes)
- [blockchain/v0] \#5741 Relax termination conditions and increase sync timeout (@melekes)
- [cli] \#5772 `gen_node_key` output now contains node ID (`id` field) (@melekes)
- [blockchain/v2] \#5774 Send status request when new peer joins (@melekes)
- [consensus] \#5792 Deprecates the `time_iota_ms` consensus parameter, to reduce the bug surface. The parameter is no longer used. (@valardragon)

### BUG FIXES

- [types] \#5523 Change json naming of `PartSetHeader` within `BlockID` from `parts` to `part_set_header` (@marbar3778)
- [privval] \#5638 Increase read/write timeout to 5s and calculate ping interval based on it (@JoeKash)
- [blockchain/v1] [\#5701](https://github.com/tendermint/tendermint/pull/5701) Handle peers without blocks (@melekes)
- [blockchain/v1] \#5711 Fix deadlock (@melekes)
