# Unreleased Changes

## v0.34.0-rc5

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES

- CLI/RPC/Config

- Apps

- P2P Protocol

- Go API

- Blockchain Protocol

### FEATURES

- [config] Add `--mode` flag and config variable. See [ADR-52](https://github.com/tendermint/tendermint/blob/master/docs/architecture/adr-052-tendermint-mode.md)

### IMPROVEMENTS

- [privval] \#5434 `NewSignerDialerEndpoint` can now be given `SignerServiceEndpointOption` (@erikgrinaker)

- [config] \#5433 `statesync.rpc_servers` is now properly set when writing the configuration file (@erikgrinaker)

### BUG FIXES

- [privval] \#5441 Fix faulty ping message encoding causing nil message errors in logs (@erikgrinaker)

