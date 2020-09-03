# v0.34.0-rc4

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

## BREAKING CHANGES

- CLI/RPC/Config
    - [config] \#5315 Rename `prof_laddr` to `pprof_laddr` and move it to `rpc` section (@melekes)
    - [rpc] \#5315 Remove `/unsafe_start_cpu_profiler`, `/unsafe_stop_cpu_profiler` and `/unsafe_write_heap_profile`. Please use pprof functionality instead (@melekes)

- Apps

    - [abci] [\#5324](https://github.com/tendermint/tendermint/pull/5324) abci evidence type is an enum with two types of possible evidence (@cmwaters)

- P2P Protocol

- Go API
    - [evidence] \#5317 Remove ConflictingHeaders evidence type & CompositeEvidence Interface. (@marbar3778)
    - [evidence] \#5318 Remove LunaticValidator evidence type. (@marbar3778)
    - [evidence] \#5319 Remove Amnesia & potentialAmnesia evidence types and removed POLC. (@marbar3778)
    - [params] \#5319 Remove `ProofofTrialPeriod` from evidence params (@marbar3778)
    - [crypto/secp256k1] \#5280 `secp256k1` has been removed from the Tendermint repo. (@marbar3778)

- Blockchain Protocol

## FEATURES

- [privval] \#5239 Add `chainID` to requests from client. (@marbar3778)
- [config] Add `--consensus.double_sign_check_height` flag and `DoubleSignCheckHeight` config variable. See [ADR-51](https://github.com/tendermint/tendermint/blob/master/docs/architecture/adr-051-double-signing-risk-reduction.md)
- [light] [\#5298](https://github.com/tendermint/tendermint/pull/5298) Morph validator set and signed header into light block (@cmwaters)

## IMPROVEMENTS

- [blockchain] \#5278 Verify only +2/3 of the signatures in a block when fast syncing. (@marbar3778)
- [rpc] \#5293 `/dial_peers` has added `private` and `unconditional` as parameters. (@marbar3778)

## BUG FIXES

- [blockchain] \#5249 Fix fast sync halt with initial height > 1 (@erikgrinaker)

- [statesync] \#5302 Fix genesis state propagation to state sync routine (@erikgrinaker)

- [statesync] \#5311 Fix validator set off-by-one causing consensus failures (@erikgrinaker)

- [statesync] \#5320 Broadcast snapshot request to all pre-connected peers on start (@erikgrinaker)

- [consensus] \#5329 Fix wrong proposer schedule for validators returned by `InitChain` (@erikgrinaker)

- [light] [\#5307](https://github.com/tendermint/tendermint/pull/5307) Persist correct proposer priority in light client validator sets (@cmwaters)
