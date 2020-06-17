## v0.33.6

\*\*

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

- CLI/RPC/Config

  - [evidence] \#4725 Remove `Pubkey` from DuplicateVoteEvidence
  - [rpc] [\#4792](https://github.com/tendermint/tendermint/pull/4792) `/validators` are now sorted by voting power (@melekes)
  - [blockchain] \#4637 Transition blockchain reactor(s) to Protobuf encoding
  - [types] \#4382  `SignedMsgType` has moved to a Protobuf enum types
  - [types] \#4382 `Total` has been changed from a `int` to a `uint32`
  - [types] \#4582 Vote: `ValidatorIndex` & `Round` are now int32
  - [types] \#4582 Proposal: `POLRound` & `Round` are now int32
  - [types] \#4582 Block: `Round` is now int32
  - [types] \#4962 `ConsensusParams`, `BlockParams`, `EvidenceParams`, `ValidatorParams` & `HashedParams` are now Protobuf types
  - [consensus] \#4582 RoundState: `Round`, `LockedRound` & `CommitRound` are now int32
  - [consensus] \#4582 HeightVoteSet: `round` is now int32
  - [privval] \#4582 `round` in private_validator_state.json is no longer a string in json it is now a number.
  - [crypto] \#4940 All keys have become `[]byte` instead of `[<size>]byte`. The byte method no longer returns the marshaled value but just the `[]byte` form of the data.
  - [crypto] \4988 Removal of key type multisig
    - The key has been moved to the Cosmos-SDK (https://github.com/cosmos/cosmos-sdk/blob/master/crypto/types/multisig/multisignature.go)
  - [crypto] \#4989 Remove `Simple` prefixes from `SimpleProof`, `SimpleValueOp` & `SimpleProofNode`. 
      - `merkle.Proof` has been renamed to `ProofOps`.
      - Protobuf messages `Proof` & `ProofOp` has been moved to `proto/crypto/merkle`
      - `SimpleHashFromByteSlices` has been renamed to `HashFromByteSlices`
      - `SimpleHashFromByteSlicesIterative` has been renamed to `HashFromByteSlicesIterative`
      - `SimpleProofsFromByteSlices` has been renamed to `ProofsFromByteSlices`
  - [crypto] \#4941 Remove suffixes from all keys.
    - ed25519: type `PrivKeyEd25519` is now `PrivKey`
    - ed25519: type `PubKeyEd25519` is now `PubKey`
    - secp256k1: type`PrivKeySecp256k1` is now `PrivKey`
    - secp256k1: type`PubKeySecp256k1` is now `PubKey`
    - sr25519: type `PrivKeySr25519` is now `PrivKey`
    - sr25519: type `PubKeySr25519` is now `PubKey`
    - multisig: type `PubKeyMultisigThreshold` is now `PubKey`
  - [light] \#4946 Rename `lite2` pkg to `light`, the lite cmd has also been renamed to `light`. Remove `lite` implementation.
  - [rpc] \#4937 Return an error when `page` pagination param is 0 in `/validators`, `tx_search` (@melekes)
  - [state] \#4679 `TxResult` is a Protobuf type defined in `abci` types directory
  - [state] \#4679 `state` reactor migration to Protobuf encoding
  - [evidence] \#4959 Add json tags to `DuplicateVoteEvidence`
  - [p2p/pex] \#4973 `p2p/pex` reactor migration to Protobuf encoding
  - [light] \#4964 `light` reactor migration to Protobuf encoding
  - [store] \#4778 Transition store module to protobuf encoding
    - `BlockStoreStateJSON` is now `BlockStoreState` and is encoded as binary in the database
  - [rpc] \#4968 JSON encoding is now handled by `libs/json`, not Amino
  - [types] \#4852 Vote & Proposal `SignBytes` is now func `VoteSignBytes` & `ProposalSignBytes`
  - [privval] \#4985 `privval` reactor migration to Protobuf encoding
  - [evidence] \#4949 `evidence` reactor migration to Protobuf encoding

- Apps

  - [abci] [\#4704](https://github.com/tendermint/tendermint/pull/4704) Add ABCI methods `ListSnapshots`, `LoadSnapshotChunk`, `OfferSnapshot`, and `ApplySnapshotChunk` for state sync snapshots. `ABCIVersion` bumped to 0.17.0.
  - [abci] \#4989 `Proof` within `ResponseQuery` has been renamed to `ProofOps`

- P2P Protocol

- Go API

  - [crypto] [\#4721](https://github.com/tendermint/tendermint/pull/4721) Remove `SimpleHashFromMap()` and `SimpleProofsFromMap()` (@erikgrinaker)
  - [types] \#4798 Simplify `VerifyCommitTrusting` func + remove extra validation (@melekes)
  - [libs] \#4831 Remove `Bech32` pkg from Tendermint. This pkg now lives in the [cosmos-sdk](https://github.com/cosmos/cosmos-sdk/tree/4173ea5ebad906dd9b45325bed69b9c655504867/types/bech32)
  - [rpc/client] \#4947 `Validators`, `TxSearch` `page`/`per_page` params become pointers (@melekes)
    `UnconfirmedTxs` `limit` param is a pointer
  - [types] \#4845 Remove `ABCIResult`

- Blockchain Protocol

  - [types] [\#4792](https://github.com/tendermint/tendermint/pull/4792) Sort validators by voting power to enable faster commit verification (@melekes)
  - [evidence] [\#4780](https://github.com/tendermint/tendermint/pull/4780) Cap evidence to an absolute number (@cmwaters)
    Add `max_num` to consensus evidence parameters (default: 50 items).
  - [mempool] \#4940 Migrate mempool from amino binary encoding to Protobuf
  - [statesync] \#4943 Migrate statesync reactor from amino binary encoding to Protobuf
  - [state] \#4845 Include BeginBlock#Events, EndBlock#Events, DeliverTx#Events, GasWanted and GasUsed into `LastResultsHash` (@melekes)

### FEATURES:

- [statesync] Add state sync support, where a new node can be rapidly bootstrapped by fetching state snapshots from peers instead of replaying blocks. See the `[statesync]` config section.
- [evidence] [\#4532](https://github.com/tendermint/tendermint/pull/4532) Handle evidence from light clients (@melekes)
- [light] [\#4532](https://github.com/tendermint/tendermint/pull/4532) Submit conflicting headers, if any, to a full node & all witnesses (@melekes)
- [rpc] [\#4532](https://github.com/tendermint/tendermint/pull/4923) Support `BlockByHash` query (@fedekunze)
- [rpc] \#4979 Support EXISTS operator in `/tx_search` query (@melekes)
- [p2p] \#4981 Expose `SaveAs` func on NodeKey (@melekes)
- [evidence] [#4821](https://github.com/tendermint/tendermint/pull/4821) Amnesia evidence can be detected, verified and committed (@cmwaters)
- [rpc] \#5017 Add `/check_tx` endpoint to check transactions without executing them or adding them to the mempool (@melekes)

### IMPROVEMENTS:

- [txindex] [\#4466](https://github.com/tendermint/tendermint/pull/4466) Allow to index an event at runtime (@favadi)
  - `abci.EventAttribute` replaces `KV.Pair`
- [evidence] [\#4722](https://github.com/tendermint/tendermint/pull/4722) Improved evidence db (@cmwaters)
- [state] [\#4781](https://github.com/tendermint/tendermint/pull/4781) Export `InitStateVersion` for the initial state version (@erikgrinaker)
- [p2p/conn] \#4795 Return err on `signChallenge()` instead of panic
- [evidence] [\#4839](https://github.com/tendermint/tendermint/pull/4839) Reject duplicate evidence from being proposed (@cmwaters)
- [evidence] [\#4892](https://github.com/tendermint/tendermint/pull/4892) Remove redundant header from phantom validator evidence (@cmwaters)
- [types] [\#4905](https://github.com/tendermint/tendermint/pull/4905) Add ValidateBasic to validator and validator set (@cmwaters)
- [consensus] [\#4578](https://github.com/tendermint/tendermint/issues/4578) Attempt to repair the consensus WAL file (`data/cs.wal/wal`) automatically in case of corruption (@alessio)
  The original WAL file will be backed up to `data/cs.wal/wal.CORRUPTED`.
- [light] [\#4935](https://github.com/tendermint/tendermint/pull/4935) Fetch and compare a new header with witnesses in parallel (@melekes)
- [light] [\#4929](https://github.com/tendermint/tendermint/pull/4929) compare header w/ witnesses only when doing bisection (@melekes)
- [light] [\#4916](https://github.com/tendermint/tendermint/pull/4916) validate basic for inbound validator sets and headers before further processing them (@cmwaters)

### BUG FIXES:

- [consensus] [\#4895](https://github.com/tendermint/tendermint/pull/4895) Cache the address of the validator to reduce querying a remote KMS (@joe-bowman)
- [consensus] \#4970 Stricter on `LastCommitRound` check (@cuonglm)
- [blockchain/v2] Correctly set block store base in status responses (@erikgrinaker)
