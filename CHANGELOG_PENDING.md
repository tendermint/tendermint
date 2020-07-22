## v0.34

\*\*

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

- CLI/RPC/Config

  - [evidence] \#4959 Add json tags to `DuplicateVoteEvidence`
  - [light] \#4946 `tendermint lite` cmd has been renamed to `tendermint light`
  - [privval] \#4582 `round` in private_validator_state.json is no longer a string in json it is now a number
  - [rpc] [\#4792](https://github.com/tendermint/tendermint/pull/4792) `/validators` are now sorted by voting power (@melekes)
  - [rpc] \#4937 Return an error when `page` pagination param is 0 in `/validators`, `tx_search` (@melekes)
  - [rpc] \#5137 The json tags of `gasWanted` & `gasUsed` in `ResponseCheckTx` & `ResponseDeliverTx` have been made snake_case. (`gas_wanted` & `gas_used`)

- Apps

  - [abci] [\#4704](https://github.com/tendermint/tendermint/pull/4704) Add ABCI methods `ListSnapshots`, `LoadSnapshotChunk`, `OfferSnapshot`, and `ApplySnapshotChunk` for state sync snapshots. `ABCIVersion` bumped to 0.17.0.
  - [abci] \#4989 `Proof` within `ResponseQuery` has been renamed to `ProofOps`
  - [abci] `CheckTxType` Protobuf enum names are now uppercase, to follow Protobuf style guide

- P2P Protocol

  - [blockchain] \#4637 Migrate blockchain reactor(s) to Protobuf encoding
  - [evidence] \#4949 Migrate evidence reactor to Protobuf encoding
  - [mempool] \#4940 Migrate mempool from to Protobuf encoding
  - [p2p/pex] \#4973 Migrate `p2p/pex` reactor to Protobuf encoding
  - [statesync] \#4943 Migrate state sync reactor to Protobuf encoding

- Blockchain Protocol

  - [evidence] [\#4780](https://github.com/tendermint/tendermint/pull/4780) Cap evidence to an absolute number (@cmwaters)
    - Add `max_num` to consensus evidence parameters (default: 50 items).
  - [evidence] \#4725 Remove `Pubkey` from `DuplicateVoteEvidence`
  - [state] \#4845 Include `BeginBlock#Events`, `EndBlock#Events`, `DeliverTx#Events`, `GasWanted` and `GasUsed` into `LastResultsHash` (@melekes)
  - [types] [\#4792](https://github.com/tendermint/tendermint/pull/4792) Sort validators by voting power to enable faster commit verification (@melekes)

- On-disk serialization

  - [state] \#4679 Migrate state module to Protobuf encoding
    - `BlockStoreStateJSON` is now `BlockStoreState` and is encoded as binary in the database
  - [store] \#4778 Migrate store module to Protobuf encoding

- Light client, private validator

  - [light] \#4964 Migrate light module migration to Protobuf encoding
  - [privval] \#4985 Migrate `privval` module to Protobuf encoding

- Go API

  - [light] \#4946 Rename `lite2` pkg to `light`. Remove `lite` implementation.
  - [crypto] [\#4721](https://github.com/tendermint/tendermint/pull/4721) Remove `SimpleHashFromMap()` and `SimpleProofsFromMap()` (@erikgrinaker)
  - [crypto] \#4940 All keys have become `[]byte` instead of `[<size>]byte`. The byte method no longer returns the marshaled value but just the `[]byte` form of the data.
  - [crypto] \4988 Removal of key type multisig
    - The key has been moved to the [Cosmos-SDK](https://github.com/cosmos/cosmos-sdk/blob/master/crypto/types/multisig/multisignature.go)
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
  - [libs] \#4831 Remove `Bech32` pkg from Tendermint. This pkg now lives in the [cosmos-sdk](https://github.com/cosmos/cosmos-sdk/tree/4173ea5ebad906dd9b45325bed69b9c655504867/types/bech32)
  - [rpc/client] \#4947 `Validators`, `TxSearch` `page`/`per_page` params become pointers (@melekes)
    - `UnconfirmedTxs` `limit` param is a pointer
  - [proto] \#5025 All proto files have been moved to `/proto` directory.
    - Using the recommended the file layout from buf, [see here for more info](https://buf.build/docs/lint-checkers#file_layout)
  - [state] \#4679 `TxResult` is a Protobuf type defined in `abci` types directory
  - [types] \#4939  `SignedMsgType` has moved to a Protobuf enum types
  - [types] \#4962 `ConsensusParams`, `BlockParams`, `EvidenceParams`, `ValidatorParams` & `HashedParams` are now Protobuf types
  - [types] \#4852 Vote & Proposal `SignBytes` is now func `VoteSignBytes` & `ProposalSignBytes`
  - [types] \#4798 Simplify `VerifyCommitTrusting` func + remove extra validation (@melekes)
  - [types] \#4845 Remove `ABCIResult`
  - [types] \#5029 Rename all values from `PartsHeader` to `PartSetHeader` to have consistency
  - [types] \#4939 `Total` in `Parts` & `PartSetHeader` has been changed from a `int` to a `uint32`
  - [types] \#4939 Vote: `ValidatorIndex` & `Round` are now `int32`
  - [types] \#4939 Proposal: `POLRound` & `Round` are now `int32`
  - [types] \#4939 Block: `Round` is now `int32`
  - [consensus] \#4582 RoundState: `Round`, `LockedRound` & `CommitRound` are now `int32`
  - [consensus] \#4582 HeightVoteSet: `round` is now `int32`
  - [rpc/jsonrpc/server] \#5141 Remove `WriteRPCResponseArrayHTTP` (use `WriteRPCResponseHTTP` instead) (@melekes)

### FEATURES:

- [abci] \#5031 Add `AppVersion` to consensus parameters (@james-ray)
  - ... making it possible to update your ABCI application version via `EndBlock` response
- [evidence] [\#4532](https://github.com/tendermint/tendermint/pull/4532) Handle evidence from light clients (@melekes)
- [evidence] [#4821](https://github.com/tendermint/tendermint/pull/4821) Amnesia evidence can be detected, verified and committed (@cmwaters)
- [light] [\#4532](https://github.com/tendermint/tendermint/pull/4532) Submit conflicting headers, if any, to a full node & all witnesses (@melekes)
- [p2p] \#4981 Expose `SaveAs` func on NodeKey (@melekes)
- [rpc] [\#4532](https://github.com/tendermint/tendermint/pull/4923) Support `BlockByHash` query (@fedekunze)
- [rpc] \#4979 Support EXISTS operator in `/tx_search` query (@melekes)
- [rpc] \#5017 Add `/check_tx` endpoint to check transactions without executing them or adding them to the mempool (@melekes)
- [statesync] Add state sync support, where a new node can be rapidly bootstrapped by fetching state snapshots from peers instead of replaying blocks. See the `[statesync]` config section.
- [rpc] [\#5108](https://github.com/tendermint/tendermint/pull/5108) Subscribe using the websocket for new evidence events (@cmwaters)

### IMPROVEMENTS:

- [consensus] [\#4578](https://github.com/tendermint/tendermint/issues/4578) Attempt to repair the consensus WAL file (`data/cs.wal/wal`) automatically in case of corruption (@alessio)
  - The original WAL file will be backed up to `data/cs.wal/wal.CORRUPTED`.
- [evidence] [\#4722](https://github.com/tendermint/tendermint/pull/4722) Improved evidence db (@cmwaters)
- [evidence] [\#4839](https://github.com/tendermint/tendermint/pull/4839) Reject duplicate evidence from being proposed (@cmwaters)
- [evidence] [\#4892](https://github.com/tendermint/tendermint/pull/4892) Remove redundant header from phantom validator evidence (@cmwaters)
- [light] [\#4935](https://github.com/tendermint/tendermint/pull/4935) Fetch and compare a new header with witnesses in parallel (@melekes)
- [light] [\#4929](https://github.com/tendermint/tendermint/pull/4929) compare header w/ witnesses only when doing bisection (@melekes)
- [light] [\#4916](https://github.com/tendermint/tendermint/pull/4916) validate basic for inbound validator sets and headers before further processing them (@cmwaters)
- [p2p/conn] \#4795 Return err on `signChallenge()` instead of panic
- [state] [\#4781](https://github.com/tendermint/tendermint/pull/4781) Export `InitStateVersion` for the initial state version (@erikgrinaker)
- [txindex] [\#4466](https://github.com/tendermint/tendermint/pull/4466) Allow to index an event at runtime (@favadi)
  - `abci.EventAttribute` replaces `KV.Pair`
- [libs] \#5126 Add a sync package which wraps sync.(RW)Mutex & deadlock.(RW)Mutex and use a build flag (deadlock) in order to enable deadlock checking
- [types] [\#4905](https://github.com/tendermint/tendermint/pull/4905) Add `ValidateBasic` to validator and validator set (@cmwaters)
- [rpc] \#4968 JSON encoding is now handled by `libs/json`, not Amino
- [mempool] Add RemoveTxByKey() exported function for custom mempool cleaning (@p4u)

### BUG FIXES:

- [blockchain/v2] Correctly set block store base in status responses (@erikgrinaker)
- [consensus] [\#4895](https://github.com/tendermint/tendermint/pull/4895) Cache the address of the validator to reduce querying a remote KMS (@joe-bowman)
- [consensus] \#4970 Stricter on `LastCommitRound` check (@cuonglm)
- [p2p][\#5136](https://github.com/tendermint/tendermint/pull/5136) Fix error for peer with the same ID but different IPs (@valardragon)
- [proxy] \#5078 Fix a bug, where TM does not exit when ABCI app crashes (@melekes)
