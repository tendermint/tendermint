## v0.34.0-rc3

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES

- Blockchain Protocol
    - [\#5193](https://github.com/tendermint/tendermint/pull/5193) Header hashes are no longer empty for empty inputs, notably `DataHash`, `EvidenceHash`, and `LastResultsHash` (@erikgrinaker)

- Go API
    - [evidence] [\#5181](https://github.com/tendermint/tendermint/pull/5181) Phantom validator evidence was removed (also from abci) (@cmwaters)  
    - [merkle] [\#5193](https://github.com/tendermint/tendermint/pull/5193) `HashFromByteSlices` and `ProofsFromByteSlices` now return a hash for empty inputs, following RFC6962 (@erikgrinaker)
    - [crypto] [\#5214] Change `GenPrivKeySecp256k1` to `GenPrivKeyFromSecret` to be consistent with other keys
    - [state] [\#5191](https://github.com/tendermint/tendermint/pull/5191) Add `State.InitialHeight` field to record initial block height, must be `1` (not `0`) to start from 1 (@erikgrinaker)
    - [state] `LoadStateFromDBOrGenesisFile()` and `LoadStateFromDBOrGenesisDoc()` no longer saves the state in the database if not found, the genesis state is simply returned (@erikgrinaker)
    - [crypto] \#5236 `VerifyBytes` is now `VerifySignature` on the `crypto.PubKey` interface.

### FEATURES:

- [abci] [\#5174](https://github.com/tendermint/tendermint/pull/5174) Add amnesia evidence and remove mock and potential amnesia evidence from abci (@cmwaters)
- [abci] [\#5191](https://github.com/tendermint/tendermint/pull/5191) Add `InitChain.InitialHeight` field giving the initial block height (@erikgrinaker)
- [abci] [\#5227](https://github.com/tendermint/tendermint/pull/5227) Add `ResponseInitChain.app_hash` which is recorded in genesis block (@erikgrinaker)
- [genesis] [\#5191](https://github.com/tendermint/tendermint/pull/5191) Add `initial_height` field to specify the initial chain height (defaults to `1`) (@erikgrinaker)
- [db] Add support for `badgerdb` database backend (@erikgrinaker)

### IMPROVEMENTS:

- [evidence] [\#5219](https://github.com/tendermint/tendermint/pull/5219) Change the source of evidence time to block time (@cmwaters)

### BUG FIXES:

- [evidence] [\#5170](https://github.com/tendermint/tendermint/pull/5170) change abci evidence time to the time the infraction happened not the time the evidence was committed on the block (@cmwaters)
- [node] Don't attempt fast sync when the ABCI application specifies ourself as the only validator via `InitChain` (@erikgrinaker)
- [libs/rand] [\#5215](https://github.com/tendermint/tendermint/pull/5215) Fix out-of-memory error on unexpected argument of Str() (@SadPencil)
