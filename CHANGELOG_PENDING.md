# Pending

BREAKING CHANGES:
- [types] Header ...
- [state] Add NextValidatorSet, changes on-disk representation of state
- [state] Validator set changes are delayed by one block (!)
- [lite] Complete refactor of the package
- [rpc] `/commit` returns a `signed_header` field instead of everything being
  top-level
- [abci] Added address of the original proposer of the block to Header.
- [abci] Change ABCI Header to match Tendermint exactly

FEATURES:

IMPROVEMENTS:

BUG FIXES:
- [mempool] No longer possible to fill up linked list without getting caching 
benefits [#2180](https://github.com/tendermint/tendermint/issues/2180)