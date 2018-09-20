# Pending

Special thanks to external contributors with PRs included in this release:

BREAKING CHANGES:

* CLI/RPC/Config

* Apps
  [rpc] /status `result.node_info.other` became a map #[2391](https://github.com/tendermint/tendermint/issues/2391)

* Go API
  * \#2310 Mempool.ReapMaxBytes -> Mempool.ReapMaxBytesMaxGas
  * \#2431 Remove Word256 code in libs/common, due to lack of use
  * \#2452 Remove the following methods from libs/common due to lack of use:
    * byteslice.go: cmn.IsZeros, cmn.RightPadBytes, cmn.LeftPadBytes, cmn.PrefixEndBytes
    * strings.go: cmn.IsHex, cmn.StripHex
    * int.go: Uint64Slice
    
* Blockchain Protocol

* P2P Protocol


FEATURES:
  * \#2310 Mempool is now aware of the MaxGas requirement

IMPROVEMENTS:
- [mempool] [\#2399](https://github.com/tendermint/tendermint/issues/2399) Make mempool cache a proper LRU (@bradyjoestar)
- [types] add Address to GenesisValidator [\#1714](https://github.com/tendermint/tendermint/issues/1714)
- [metrics] `consensus.block_interval_metrics` is now gauge, not histogram (you will be able to see spikes, if any)

BUG FIXES:
- [node] \#2294 Delay starting node until Genesis time
