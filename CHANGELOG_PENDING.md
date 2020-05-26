## v0.33.5


\*\*

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

- CLI/RPC/Config

- Apps

- P2P Protocol

- Go API

  - [privval] [\#4744](https://github.com/tendermint/tendermint/pull/4744) Remove deprecated `OldFilePV` (@melekes)

- Blockchain Protocol

### FEATURES:


### IMPROVEMENTS:

- [abci/server] [\#4719](https://github.com/tendermint/tendermint/pull/4719) Print panic & stack trace to STDERR if logger is not set (@melekes)
- [types] [\#4638](https://github.com/tendermint/tendermint/pull/4638) Implement `Header#ValidateBasic` (@alexanderbez)
- [buildsystem] [\#4378](https://github.com/tendermint/tendermint/pull/4738) Replace build_c and install_c with TENDERMINT_BUILD_OPTIONS parsing. The following options are available:
  - nostrip: don't strip debugging symbols nor DWARF tables.
  - cleveldb: use cleveldb as db backend instead of goleveldb.
  - race: pass -race to go build and enable data race detection.

### BUG FIXES:

- [light] [\#4741](https://github.com/tendermint/tendermint/pull/4741) Correctly return  `ErrSignedHeaderNotFound` and `ErrValidatorSetNotFound` on corresponding RPC errors (@erikgrinaker)
