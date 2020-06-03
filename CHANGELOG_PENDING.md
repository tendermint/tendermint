## v0.33.6

\*\*

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

- CLI/RPC/Config

  - [evidence] \#4725 Remove `Pubkey` from DuplicateVoteEvidence
  - [rpc] [\#4792](https://github.com/tendermint/tendermint/pull/4792) `/validators` are now sorted by voting power (@melekes)
  - [crypto] \#4941 Remove suffixes from all keys.
    - ed25519: type `PrivKeyEd25519` is now `PrivKey`
    - ed25519: type `PubKeyEd25519` is now `PubKey`
    - secp256k1: type`PrivKeySecp256k1` is now `PrivKey`
    - secp256k1: type`PubKeySecp256k1` is now `PubKey`
    - sr25519: type `PrivKeySr25519` is now `PrivKey`
    - sr25519: type `PubKeySr25519` is now `PubKey`
    - multisig: type `PubKeyMultisigThreshold` is now `PubKey`
  - [light] \#4946 Rename `lite2` pkg to `light`, the lite cmd has also been renamed to `light`. Remove `lite` implementation.

- Apps

  - [abci] [\#4704](https://github.com/tendermint/tendermint/pull/4704) Add ABCI methods `ListSnapshots`, `LoadSnapshotChunk`, `OfferSnapshot`, and `ApplySnapshotChunk` for state sync snapshots. `ABCIVersion` bumped to 0.17.0.

- P2P Protocol

- Go API

  - [crypto] [\#4721](https://github.com/tendermint/tendermint/pull/4721) Remove `SimpleHashFromMap()` and `SimpleProofsFromMap()` (@erikgrinaker)
  - [types] \#4798 Simplify `VerifyCommitTrusting` func + remove extra validation (@melekes)
  - [libs] \#4831 Remove `Bech32` pkg from Tendermint. This pkg now lives in the [cosmos-sdk](https://github.com/cosmos/cosmos-sdk/tree/4173ea5ebad906dd9b45325bed69b9c655504867/types/bech32)
- Blockchain Protocol

  - [types] [\#4792](https://github.com/tendermint/tendermint/pull/4792) Sort validators by voting power to enable faster commit verification (@melekes)
  - [evidence] [\#4780](https://github.com/tendermint/tendermint/pull/4780) Cap evidence to an absolute number (@cmwaters)
    Add `max_num` to consensus evidence parameters (default: 50 items).
  - [mempool] \#4940 Migrate mempool from amino binary encoding to Protobuf
  - [statesync] \#4943 Migrate statesync reactor from amino binary encoding to Protobuf

### FEATURES:

- [statesync] Add state sync support, where a new node can be rapidly bootstrapped by fetching state snapshots from peers instead of replaying blocks. See the `[statesync]` config section.
- [evidence] [\#4532](https://github.com/tendermint/tendermint/pull/4532) Handle evidence from light clients (@melekes)
- [lite2] [\#4532](https://github.com/tendermint/tendermint/pull/4532) Submit conflicting headers, if any, to a full node & all witnesses (@melekes)
- [rpc] [\#4532](https://github.com/tendermint/tendermint/pull/4923) Support `BlockByHash` query (@fedekunze)

### IMPROVEMENTS:

- [txindex] [\#4466](https://github.com/tendermint/tendermint/pull/4466) Allow to index an event at runtime (@favadi)
  - `abci.EventAttribute` replaces `KV.Pair`
- [evidence] [\#4722](https://github.com/tendermint/tendermint/pull/4722) Improved evidence db (@cmwaters)
- [state] [\#4781](https://github.com/tendermint/tendermint/pull/4781) Export `InitStateVersion` for the initial state version (@erikgrinaker)
- [p2p/conn] \#4795 Return err on `signChallenge()` instead of panic
- [evidence] [\#4839](https://github.com/tendermint/tendermint/pull/4839) Reject duplicate evidence from being proposed (@cmwaters)
- [evidence] [\#4892](https://github.com/tendermint/tendermint/pull/4892) Remove redundant header from phantom validator evidence (@cmwaters)
- [types] [\#4905](https://github.com/tendermint/tendermint/pull/4905) Add ValidateBasic to validator and validator set (@cmwaters)
- [lite2] [\#4935](https://github.com/tendermint/tendermint/pull/4935) Fetch and compare a new header with witnesses in parallel (@melekes)
- [lite2] [\#4929](https://github.com/tendermint/tendermint/pull/4929) compare header w/ witnesses only when doing bisection (@melekes)

### BUG FIXES:

- [consensus] [\#4895](https://github.com/tendermint/tendermint/pull/4895) Cache the address of the validator to reduce querying a remote KMS (@joe-bowman)
