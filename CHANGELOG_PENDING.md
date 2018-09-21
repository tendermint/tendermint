# Pending

Special thanks to external contributors on this release:
@scriptionist, @bradyjoestar, @WALL-E

This release is mostly about the ConsensusParams - removing fields and enforcing MaxGas.
It also addresses some issues found via security audit, removes various unused
functions from `libs/common`, and implements
[ADR-012](https://github.com/tendermint/tendermint/blob/develop/docs/architecture/adr-012-peer-transport.md).

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

BREAKING CHANGES:

* CLI/RPC/Config
  * [rpc] [\#2391](https://github.com/tendermint/tendermint/issues/2391) /status `result.node_info.other` became a map
  * [types] [\#2364](https://github.com/tendermint/tendermint/issues/2364) Remove `TxSize` and `BlockGossip` from `ConsensusParams`
    * Maximum tx size is now set implicitly via the `BlockSize.MaxBytes`
    * The size of block parts in the consensus is now fixed to 64kB

* Apps
  * [mempool] [\#2360](https://github.com/tendermint/tendermint/issues/2360) Mempool tracks the `ResponseCheckTx.GasWanted` and enforces `ConsensusParams.BlockSize.MaxGas` on proposals.

* Go API
  * [libs/common] [\#2431](https://github.com/tendermint/tendermint/issues/2431) Remove Word256 due to lack of use
  * [libs/common] [\#2452](https://github.com/tendermint/tendermint/issues/2452) Remove the following functions due to lack of use:
    * byteslice.go: cmn.IsZeros, cmn.RightPadBytes, cmn.LeftPadBytes, cmn.PrefixEndBytes
    * strings.go: cmn.IsHex, cmn.StripHex
    * int.go: Uint64Slice, all put/get int64 methods

FEATURES:
- [rpc] [\#2415](https://github.com/tendermint/tendermint/issues/2415) New `/consensus_params?height=X` endpoint to query the consensus
  params at any height (@scriptonist)
- [types] [\#1714](https://github.com/tendermint/tendermint/issues/1714) Add Address to GenesisValidator
- [metrics] [\#2337](https://github.com/tendermint/tendermint/issues/2337) `consensus.block_interval_metrics` is now gauge, not histogram (you will be able to see spikes, if any)

IMPROVEMENTS:
- [libs/db] [\#2371](https://github.com/tendermint/tendermint/issues/2371) Output error instead of panic when the given `db_backend` is not initialised (@bradyjoestar)
- [libs] [\#2286](https://github.com/tendermint/tendermint/issues/2286) Enforce 0600 permissions on `autofile` and `db/fsdb`

- [mempool] [\#2399](https://github.com/tendermint/tendermint/issues/2399) Make mempool cache a proper LRU (@bradyjoestar)
- [p2p] [\#2126](https://github.com/tendermint/tendermint/issues/2126) Introduce PeerTransport interface to improve isolation of concerns
- [libs/common] [\#2326](https://github.com/tendermint/tendermint/issues/2326) Service returns ErrNotStarted

BUG FIXES:
- [node] [\#2294](https://github.com/tendermint/tendermint/issues/2294) Delay starting node until Genesis time
- [consensus] [\#2048](https://github.com/tendermint/tendermint/issues/2048) Correct peer statistics for marking peer as good
- [rpc] [\#2460](https://github.com/tendermint/tendermint/issues/2460) StartHTTPAndTLSServer() now passes StartTLS() errors back to the caller rather than hanging forever.
- [p2p] [\#2047](https://github.com/tendermint/tendermint/issues/2047) Accept new connections asynchronously
- [tm-bench] [\#2410](https://github.com/tendermint/tendermint/issues/2410) Enforce minimum transaction size (@WALL-E)
