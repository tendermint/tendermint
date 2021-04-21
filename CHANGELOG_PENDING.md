# Unreleased Changes

## v0.34.11

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES

- CLI/RPC/Config

- Apps

- P2P Protocol

- Go API
<<<<<<< HEAD
=======
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
>>>>>>> d36a5905a... statesync: improve e2e test outcomes (#6378)

- Blockchain Protocol

### FEATURES

### IMPROVEMENTS

<<<<<<< HEAD
=======
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

>>>>>>> d36a5905a... statesync: improve e2e test outcomes (#6378)
### BUG FIXES
