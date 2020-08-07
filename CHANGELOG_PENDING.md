## v0.34.1

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES

- Go API

    - [evidence] [\#5181](https://github.com/tendermint/tendermint/pull/5181) Phantom validator evidence was removed (also from abci) (@cmwaters)  
    - [state] [\#5191](https://github.com/tendermint/tendermint/pull/5191/files) Add `State.InitialHeight` field to record initial block height, must be `1` (not `0`) to start from 1 (@erikgrinaker)
    - [state] [\#5191](https://github.com/tendermint/tendermint/pull/5191/files) `ExecCommitBlock()` now takes an additional parameter for the initial height, must be `1` if starting from 1 (@erikgrinaker)

### FEATURES:

- [abci] [\#5174](https://github.com/tendermint/tendermint/pull/5174) Add amnesia evidence and remove mock and potential amnesia evidence from abci (@cmwaters)
- [abci] [\#5191](https://github.com/tendermint/tendermint/pull/5191/files) Add `InitChain.InitialHeight` field giving the initial block height (@erikgrinaker)
- [genesis] [\#5191](https://github.com/tendermint/tendermint/pull/5191/files) Add `initial_height` field to specify the initial chain height (defaults to `1`) (@erikgrinaker)

### BUG FIXES:

- [evidence] [\#5170](https://github.com/tendermint/tendermint/pull/5170) change abci evidence time to the time the infraction happened not the time the evidence was committed on the block (@cmwaters)
