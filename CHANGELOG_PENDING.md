# Pending

BREAKING CHANGES:
- [types] CanonicalTime uses nanoseconds instead of clipping to ms
    - breaks serialization/signing of all messages with a timestamp
- [abci] Removed Fee from ResponseDeliverTx and ResponseCheckTx
- [tools] Removed `make ensure_deps` in favor of `make get_vendor_deps`
- [p2p] Remove salsa and ripemd primitives, in favor of using chacha as a stream cipher, and hkdf
- [abci] Changed time format from int64 to google.protobuf.Timestamp
- [abci] Changed Validators to LastCommitInfo in RequestBeginBlock

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

BUG FIXES:
- [common] \#2109 Safely handle cases where atomic write files already exist
- [privval] fix a deadline for accepting new connections in socket private
  validator.
