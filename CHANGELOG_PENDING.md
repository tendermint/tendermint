# Unreleased Changes

## v0.34.0-rc5

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES

- CLI/RPC/Config

- Apps
    - [ABCI] \#5447 Remove `SetOption` method from `ABCI.Client` interface

- P2P Protocol

- Go API
    - [evidence] [\#5499](https://github.com/tendermint/tendermint/pull/5449) `MaxNum` evidence consensus parameter has been changed to `MaxBytes` (@cmwaters)

- Blockchain Protocol

### FEATURES

### IMPROVEMENTS

- [privval] \#5434 `NewSignerDialerEndpoint` can now be given `SignerServiceEndpointOption` (@erikgrinaker)

- [config] \#5433 `statesync.rpc_servers` is now properly set when writing the configuration file (@erikgrinaker)

### BUG FIXES

- [privval] \#5441 Fix faulty ping message encoding causing nil message errors in logs (@erikgrinaker)
