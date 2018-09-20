# Pending

Special thanks to external contributors with PRs included in this release:

BREAKING CHANGES:

* CLI/RPC/Config
  * [rpc] [\#2391](https://github.com/tendermint/tendermint/issues/2391) /status `result.node_info.other` became a map

* Apps
  * [mempool] \#2310 Mempool tracks the `ResponseCheckTx.GasWanted` and enforces `ConsensusParams.BlockSize.MaxGas` on proposals.

* Go API
  * [libs/common] \#2431 Remove Word256 due to lack of use
  * [libs/common] \#2452 Remove the following functions due to lack of use:
    * byteslice.go: cmn.IsZeros, cmn.RightPadBytes, cmn.LeftPadBytes, cmn.PrefixEndBytes
    * strings.go: cmn.IsHex, cmn.StripHex
    * int.go: Uint64Slice, all put/get int64 methods

* Blockchain Protocol

* P2P Protocol


FEATURES:

IMPROVEMENTS:
- [libs/db] \#2371 Output error instead of panic when the given db_backend is not initialised (@bradyjoestar)
- [mempool] [\#2399](https://github.com/tendermint/tendermint/issues/2399) Make mempool cache a proper LRU (@bradyjoestar)
- [types] [\#1714](https://github.com/tendermint/tendermint/issues/1714) Add Address to GenesisValidator
- [metrics] `consensus.block_interval_metrics` is now gauge, not histogram (you will be able to see spikes, if any)
- [p2p] \#2126 Introduce PeerTransport interface to improve isolation of concerns

BUG FIXES:
- [node] \#2294 Delay starting node until Genesis time
