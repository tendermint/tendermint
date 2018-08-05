# Pending

BREAKING CHANGES:
- [types] CanonicalTime uses nanoseconds instead of clipping to ms
    - breaks serialization/signing of all messages with a timestamp
- [types] Header ...
- [state] Add NextValidatorSet, changes on-disk representation of state
- [state] Validator set changes are delayed by one block (!)
- [lite] Complete refactor of the package
- [rpc] `/commit` returns a `signed_header` field instead of everything being
  top-level
- [abci] Removed Fee from ResponseDeliverTx and ResponseCheckTx
- [tools] Removed `make ensure_deps` in favor of `make get_vendor_deps`
- [p2p] Remove salsa and ripemd primitives, in favor of using chacha as a stream cipher, and hkdf
- [abci] Changed time format from int64 to google.protobuf.Timestamp
- [abci] Changed Validators to LastCommitInfo in RequestBeginBlock
- [crypto] Switch crypto.Signature from interface to []byte for space efficiency

FEATURES:
- [tools] Added `make check_dep`
    - ensures gopkg.lock is synced with gopkg.toml
    - ensures no branches are used in the gopkg.toml

IMPROVEMENTS:
- [blockchain] Improve fast-sync logic
    - tweak params
    - only process one block at a time to avoid starving
- [crypto] Switch hkdfchachapoly1305 to xchachapoly1305
- [common] bit array functions which take in another parameter are now thread safe
- [p2p] begin connecting to peers as soon a seed node provides them to you ([#2093](https://github.com/tendermint/tendermint/issues/2093))

BUG FIXES:
- [common] Safely handle cases where atomic write files already exist [#2109](https://github.com/tendermint/tendermint/issues/2109)
- [privval] fix a deadline for accepting new connections in socket private
  validator.
- [p2p] Allow startup if a configured seed node's IP can't be resolved ([#1716](https://github.com/tendermint/tendermint/issues/1716))
- [node] Fully exit when CTRL-C is pressed even if consensus state panics [#2072](https://github.com/tendermint/tendermint/issues/2072)
