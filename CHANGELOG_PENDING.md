## v0.34.1

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES

- Go API

    - [evidence] [\#5181](https://github.com/tendermint/tendermint/pull/5181) Phantom validator evidence was removed (also from abci) (@cmwaters) 
    - [crypto] [\#5214] Change `GenPrivKeySecp256k1` to `GenPrivKeyFromSecret` to be consistent with other keys

### FEATURES:

- [abci] [\#5174](https://github.com/tendermint/tendermint/pull/5174) Add amnesia evidence and remove mock and potential amnesia evidence from abci (@cmwaters)

### BUG FIXES:

- [evidence] [\#5170](https://github.com/tendermint/tendermint/pull/5170) change abci evidence time to the time the infraction happened not the time the evidence was committed on the block (@cmwaters)
- [node] Don't attempt fast sync when the ABCI application specifies ourself as the only validator via `InitChain` (@erikgrinaker)
