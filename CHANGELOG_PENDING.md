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
- [libs] Remove cmn.Fmt, in favor of fmt.Sprintf

FEATURES:

IMPROVEMENTS:

BUG FIXES:
