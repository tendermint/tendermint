# Pending

Special thanks to external contributors with PRs included in this release:

BREAKING CHANGES:
- [types] Header ...
- [state] Add NextValidatorSet, changes on-disk representation of state
- [state] Validator set changes are delayed by one block (!)
- [lite] Complete refactor of the package
- [rpc] `/commit` returns a `signed_header` field instead of everything being
  top-level
- [abci] Added address of the original proposer of the block to Header.
- [abci] Change ABCI Header to match Tendermint exactly
- [common] SplitAndTrim was deleted

FEATURES:
  * \#2310 Mempool is now aware of the MaxGas requirement

IMPROVEMENTS:
- [mempool] [\#2399](https://github.com/tendermint/tendermint/issues/2399) Make mempool cache a proper LRU (@bradyjoestar)
- [types] add Address to GenesisValidator [\#1714](https://github.com/tendermint/tendermint/issues/1714)
- [metrics] `consensus.block_interval_metrics` is now gauge, not histogram (you will be able to see spikes, if any)

BUG FIXES:
- [node] \#2294 Delay starting node until Genesis time
