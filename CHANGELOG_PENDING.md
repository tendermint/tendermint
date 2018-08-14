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
- [crypto] Rename AminoRoute variables to no longer be prefixed by signature type.
- [config] Replace MaxNumPeers with MaxNumInboundPeers and MaxNumOutboundPeers

FEATURES:

IMPROVEMENTS:
- [scripts] Added json2wal tool, which is supposed to help our users restore
  corrupted WAL files and compose test WAL files (@bradyjoestar)

BUG FIXES:
- [mempool] No longer possible to fill up linked list without getting caching 
benefits [#2180](https://github.com/tendermint/tendermint/issues/2180)