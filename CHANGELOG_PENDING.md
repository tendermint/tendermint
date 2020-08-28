# v0.34.0-rc4

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

## BREAKING CHANGES

- [crypto/secp256k1] \#5280 `secp256k1` has been removed from the Tendermint repo. (@marbar3778)

## FEATURES

- [privval] \#5239 Add `chainID` to requests from client. (@marbar3778)
- [config] Add `--consensus.double_sign_check_height` flag and `DoubleSignCheckHeight` config variable. See [ADR-51](https://github.com/tendermint/tendermint/blob/master/docs/architecture/adr-051-double-signing-risk-reduction.md)

## IMPROVEMENTS

- [blockchain] \#5278 Verify only +2/3 of the signatures in a block when fast syncing. (@marbar3778)

## BUG FIXES

- [blockchain] \#5249 Fix fast sync halt with initial height > 1 (@erikgrinaker)

- [statesync] \#5302 Fix genesis state propagation to state sync routine (@erikgrinaker)
