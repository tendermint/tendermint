# Pending

BREAKING CHANGES:

* CLI/RPC/Config
  - [config] [\#2169](https://github.com/tendermint/tendermint/issues/2169) Replace MaxNumPeers with MaxNumInboundPeers and MaxNumOutboundPeers
  - [config] [\#2300](https://github.com/tendermint/tendermint/issues/2300) Reduce default mempool size from 100k to 5k, until ABCI rechecking is implemented.
  - [rpc] [\#1815](https://github.com/tendermint/tendermint/issues/1815) `/commit` returns a `signed_header` field instead of everything being top-level

* Apps
  - [abci] Added address of the original proposer of the block to Header
  - [abci] Change ABCI Header to match Tendermint exactly
  - [abci] [\#2159](https://github.com/tendermint/tendermint/issues/2159) Update use of `Validator` (see
    [ADR-018](https://github.com/tendermint/tendermint/blob/develop/docs/architecture/adr-018-ABCI-Validators.md)):
    - Remove PubKey from `Validator` (so it's just Address and Power)
    - Introduce `ValidatorUpdate` (with just PubKey and Power)
    - InitChain and EndBlock use ValidatorUpdate
    - Update field names and types in BeginBlock
  - [state] [\#1815](https://github.com/tendermint/tendermint/issues/1815) Validator set changes are now delayed by one block
    - updates returned in ResponseEndBlock for block H will be included in RequestBeginBlock for block H+2

* Go API
  - [lite] [\#1815](https://github.com/tendermint/tendermint/issues/1815) Complete refactor of the package
  - [node] [\#2212](https://github.com/tendermint/tendermint/issues/2212) NewNode now accepts a `*p2p.NodeKey`
  - [libs/common] [\#2199](https://github.com/tendermint/tendermint/issues/2199) Remove Fmt, in favor of fmt.Sprintf
  - [libs/common] SplitAndTrim was deleted
  - [libs/clist] Panics if list extends beyond MaxLength
  - [crypto] [\#2205](https://github.com/tendermint/tendermint/issues/2205) Rename AminoRoute variables to no longer be prefixed by signature type.

* Blockchain Protocol
  - [state] [\#1815](https://github.com/tendermint/tendermint/issues/1815) Validator set changes are now delayed by one block (!)
    - Add NextValidatorSet to State, changes on-disk representation of state
  - [state] [\#2184](https://github.com/tendermint/tendermint/issues/2184) Enforce ConsensusParams.BlockSize.MaxBytes (See
    [ADR-020](https://github.com/tendermint/tendermint/blob/develop/docs/architecture/adr-020-block-size.md)).
    - Remove ConsensusParams.BlockSize.MaxTxs
    - Introduce maximum sizes for all components of a block, including ChainID
  - [types] Updates to the block Header:
    - [\#1815](https://github.com/tendermint/tendermint/issues/1815) NextValidatorsHash - hash of the validator set for the next block,
      so the current validators actually sign over the hash for the new
      validators
    - [\#2106](https://github.com/tendermint/tendermint/issues/2106) ProposerAddress - address of the block's original proposer
  - [consensus] [\#2203](https://github.com/tendermint/tendermint/issues/2203) Implement BFT time
    - Timestamp in block must be monotonic and equal the median of timestamps in block's LastCommit
  - [crypto] [\#2239](https://github.com/tendermint/tendermint/issues/2239) Secp256k1 signature changes (See
    [ADR-014](https://github.com/tendermint/tendermint/blob/develop/docs/architecture/adr-014-secp-malleability.md)):
    - format changed from DER to `r || s`, both little endian encoded as 32 bytes.
    - malleability removed by requiring `s` to be in canonical form.

* P2P Protocol
  - [p2p] [\#2263](https://github.com/tendermint/tendermint/issues/2263) Update secret connection to use a little endian encoded nonce
  - [blockchain] [\#2213](https://github.com/tendermint/tendermint/issues/2213) Fix Amino routes for blockchain reactor messages


FEATURES:
- [types] [\#2015](https://github.com/tendermint/tendermint/issues/2015) Allow genesis file to have 0 validators
  - Initial validator set can be determined by the app in ResponseInitChain
- [rpc] [\#2161](https://github.com/tendermint/tendermint/issues/2161) New event `ValidatorSetUpdates` for when the validator set changes
- [crypto/multisig] [\#2164](https://github.com/tendermint/tendermint/issues/2164) Introduce multisig pubkey and signature format
- [libs/db] [\#2293](https://github.com/tendermint/tendermint/issues/2293) Allow passing options through when creating instances of leveldb dbs

IMPROVEMENTS:
- [docs] Lint documentation with `write-good` and `stop-words`.
- [scripts] Added json2wal tool, which is supposed to help our users restore
  corrupted WAL files and compose test WAL files (@bradyjoestar)
- [mempool] [\#2234](https://github.com/tendermint/tendermint/issues/2234) Now stores txs by hash inside of the cache, to mitigate memory leakage

BUG FIXES:
- [config] [\#2284](https://github.com/tendermint/tendermint/issues/2284) Replace `db_path` with `db_dir` from automatically generated configuration files.
- [mempool] [\#2188](https://github.com/tendermint/tendermint/issues/2188) Fix OOM issue from cache map and list getting out of sync
- [state] [\#2051](https://github.com/tendermint/tendermint/issues/2051) KV store index supports searching by `tx.height`
- [rpc] [\#2327](https://github.com/tendermint/tendermint/issues/2327) `/dial_peers` does not try to dial existing peers
- [node] [\#2323](https://github.com/tendermint/tendermint/issues/2323) Filter empty strings from config lists
- [abci/client] [\#2236](https://github.com/tendermint/tendermint/issues/2236) Fix closing GRPC connection
