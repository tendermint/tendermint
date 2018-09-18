# Pending

Special thanks to external contributors with PRs included in this release:

BREAKING CHANGES:

* CLI/RPC/Config

* Apps
  [rpc] /status `result.node_info.other` became a map #[2391](https://github.com/tendermint/tendermint/issues/2391)

* Go API
  * \#2310 Mempool.ReapMaxBytes -> Mempool.ReapMaxBytesMaxGas
* Blockchain Protocol

* P2P Protocol


FEATURES:
  * \#2310 Mempool is now aware of the MaxGas requirement

IMPROVEMENTS:
- [types] add Address to GenesisValidator [\#1714](https://github.com/tendermint/tendermint/issues/1714)

BUG FIXES:
