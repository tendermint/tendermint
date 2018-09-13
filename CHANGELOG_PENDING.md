# Pending

Special thanks to external contributors with PRs included in this release:

BREAKING CHANGES:

* CLI/RPC/Config

* Apps

* Go API
  * \#2310 Mempool.ReapMaxBytes -> Mempool.ReapMaxBytesMaxGas
* Blockchain Protocol

* P2P Protocol


FEATURES:
  * \#2310 Mempool is now aware of the MaxGas requirement

IMPROVEMENTS:

BUG FIXES:
- [abci/client] [\#2265](https://github.com/tendermint/tendermint/issues/2265) CTRL-C doesn't work while waiting for ABCI app to connect(@bradyjoestar)
