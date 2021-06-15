# Unreleased Changes

## v0.34.11

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES

- CLI/RPC/Config

- Apps

    - [Version] \#6494 `TMCoreSemVer` is not required to be set as a ldflag any longer.

- P2P Protocol

- Go API

- Blockchain Protocol

### FEATURES

### IMPROVEMENTS

<<<<<<< HEAD
=======
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
>>>>>>> 7d961b55b (state sync: tune request timeout and chunkers (#6566))
- [statesync] \#6378 Retry requests for snapshots and add a minimum discovery time (5s) for new snapshots.

### BUG FIXES

- [evidence] \#6375 Fix bug with inconsistent LightClientAttackEvidence hashing (cmwaters)
