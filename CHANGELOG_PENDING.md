# Unreleased Changes

## v0.34.11

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES

- CLI/RPC/Config

- Apps

- P2P Protocol

- Go API

- Blockchain Protocol

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
- [store] \#5888 store.SaveBlock saves using batches instead of transactions for now to improve ACID properties. This is a quick fix for underlying issues around tm-db and ACID guarantees. (@githubsands)
- [consensus] \#5987 Remove `time_iota_ms` from consensus params. Merge `tmproto.ConsensusParams` and `abci.ConsensusParams`. (@marbar3778)
- [types] \#5994 Reduce the use of protobuf types in core logic. (@marbar3778)
  - `ConsensusParams`, `BlockParams`, `ValidatorParams`, `EvidenceParams`, `VersionParams`, `sm.Version` and `version.Consensus` have become native types. They still utilize protobuf when being sent over the wire or written to disk.
- [rpc/client/http] \#6163 Do not drop events even if the `out` channel is full (@melekes)
- [node] \#6059 Validate and complete genesis doc before saving to state store (@silasdavis)
- [state] \#6067 Batch save state data (@githubsands & @cmwaters)
- [libs/log] \#6174 Include timestamp (`ts` field; `time.RFC3339Nano` format) in JSON logger output (@melekes)

### BUG FIXES
