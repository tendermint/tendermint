# Data Structures

Here we describe the data structures in the Tendermint blockchain and the rules for validating them.

The Tendermint blockchains consists of a short list of data types:

- [Data Structures](#data-structures)
  - [Block](#block)
  - [Execution](#execution)
  - [Header](#header)
  - [Version](#version)
  - [BlockID](#blockid)
  - [PartSetHeader](#partsetheader)
  - [Part](#part)
  - [Time](#time)
  - [Data](#data)
  - [Commit](#commit)
  - [CommitSig](#commitsig)
  - [BlockIDFlag](#blockidflag)
  - [Vote](#vote)
  - [CanonicalVote](#canonicalvote)
  - [Proposal](#proposal)
  - [SignedMsgType](#signedmsgtype)
  - [Signature](#signature)
  - [EvidenceList](#evidencelist)
  - [Evidence](#evidence)
    - [DuplicateVoteEvidence](#duplicatevoteevidence)
    - [LightClientAttackEvidence](#lightclientattackevidence)
  - [LightBlock](#lightblock)
  - [SignedHeader](#signedheader)
  - [ValidatorSet](#validatorset)
  - [Validator](#validator)
  - [Address](#address)
  - [ConsensusParams](#consensusparams)
    - [BlockParams](#blockparams)
    - [EvidenceParams](#evidenceparams)
    - [ValidatorParams](#validatorparams)
    - [VersionParams](#versionparams)
    - [SynchronyParams](#synchronyparams)
    - [TimeoutParams](#timeoutparams)
  - [Proof](#proof)

## Block

A block consists of a header, transactions, votes (the commit),
and a list of evidence of malfeasance (ie. signing conflicting votes).

| Name   | Type              | Description                                                                                                                                                                          | Validation                                               |
|--------|-------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------------------------------------------------|
| Header | [Header](#header) | Header corresponding to the block. This field contains information used throughout consensus and other areas of the protocol. To find out what it contains, visit [header] (#header) | Must adhere to the validation rules of [header](#header) |
| Data       | [Data](#data)                  | Data contains a list of transactions. The contents of the transaction is unknown to Tendermint.                                                                                      | This field can be empty or populated, but no validation is performed. Applications can perform validation on individual transactions prior to block creation using [checkTx](../abci/abci.md#checktx).
| Evidence   | [EvidenceList](#evidencelist) | Evidence contains a list of infractions committed by validators.                                                                                                                     | Can be empty, but when populated the validations rules from [evidenceList](#evidencelist) apply |
| LastCommit | [Commit](#commit)              | `LastCommit` includes one vote for every validator.  All votes must either be for the previous block, nil or absent. If a vote is for the previous block it must have a valid signature from the corresponding validator. The sum of the voting power of the validators that voted must be greater than 2/3 of the total voting power of the complete validator set. The number of votes in a commit is limited to 10000 (see `types.MaxVotesCount`).                                                                                             | Must be empty for the initial height and must adhere to the validation rules of [commit](#commit).  |

## Execution

Once a block is validated, it can be executed against the state.

The state follows this recursive equation:

```go
state(initialHeight) = InitialState
state(h+1) <- Execute(state(h), ABCIApp, block(h))
```

where `InitialState` includes the initial consensus parameters and validator set,
and `ABCIApp` is an ABCI application that can return results and changes to the validator
set (TODO). Execute is defined as:

```go
func Execute(s State, app ABCIApp, block Block) State {
 // Fuction ApplyBlock executes block of transactions against the app and returns the new root hash of the app state,
 // modifications to the validator set and the changes of the consensus parameters.
 AppHash, ValidatorChanges, ConsensusParamChanges := app.ApplyBlock(block)

 nextConsensusParams := UpdateConsensusParams(state.ConsensusParams, ConsensusParamChanges)
 return State{
  ChainID:         state.ChainID,
  InitialHeight:   state.InitialHeight,
  LastResults:     abciResponses.DeliverTxResults,
  AppHash:         AppHash,
  InitialHeight:   state.InitialHeight,
  LastValidators:  state.Validators,
  Validators:      state.NextValidators,
  NextValidators:  UpdateValidators(state.NextValidators, ValidatorChanges),
  ConsensusParams: nextConsensusParams,
  Version: {
   Consensus: {
    AppVersion: nextConsensusParams.Version.AppVersion,
   },
  },
 }
}
```

Validating a new block is first done prior to the `prevote`, `precommit` & `finalizeCommit` stages.

The steps to validate a new block are:

- Check the validity rules of the block and its fields.
- Check the versions (Block & App) are the same as in local state.
- Check the chainID's match.
- Check the height is correct.
- Check the `LastBlockID` corresponds to BlockID currently in state.
- Check the hashes in the header match those in state.
- Verify the LastCommit against state, this step is skipped for the initial height.
    - This is where checking the signatures correspond to the correct block will be made.
- Make sure the proposer is part of the validator set.
- Validate bock time.
    - Make sure the new blocks time is after the previous blocks time.
    - Calculate the medianTime and check it against the blocks time.
    - If the blocks height is the initial height then check if it matches the genesis time.
- Validate the evidence in the block. Note: Evidence can be empty

## Header

A block header contains metadata about the block and about the consensus, as well as commitments to
the data in the current block, the previous block, and the results returned by the application:

| Name              | Type                      | Description                                                                                                                                                                                                                                                                                                                                                                           | Validation                                                                                                                                                                                       |
|-------------------|---------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Version           | [Version](#version)       | Version defines the application and protocol version being used.                                                                                                                                                                                                                                                                                                                       | Must adhere to the validation rules of [Version](#version)                                                                                                                                       |
| ChainID           | String                    | ChainID is the ID of the chain. This must be unique to your chain.                                                                                                                                                                                                                                                                                                                    | ChainID must be less than 50 bytes.                                                                                                                                                              |
| Height            | uint64                     | Height is the height for this header.                                                                                                                                                                                                                                                                                                                                                 | Must be > 0, >= initialHeight, and == previous Height+1                                                                                                                                          |
| Time              | [Time](#time)             | The timestamp is equal to the weighted median of validators present in the last commit. Read more on time in the [BFT-time section](../consensus/bft-time.md). Note: the timestamp of a vote must be greater by at least one millisecond than that of the block being voted on.                                                                                                       | Time must be >= previous header timestamp + consensus parameters TimeIotaMs.  The timestamp of the first block must be equal to the genesis time (since there's no votes to compute the median). |
| LastBlockID       | [BlockID](#blockid)       | BlockID of the previous block.                                                                                                                                                                                                                                                                                                                                                        | Must adhere to the validation rules of [blockID](#blockid). The first block has `block.Header.LastBlockID == BlockID{}`.                                                                         |
| LastCommitHash    | slice of bytes (`[]byte`) | MerkleRoot of the lastCommit's signatures. The signatures represent the validators that committed to the last block. The first block has an empty slices of bytes for the hash.                                                                                                                                                                                                       | Must  be of length 32                                                                                                                                                                            |
| DataHash          | slice of bytes (`[]byte`) | MerkleRoot of the hash of transactions. **Note**: The transactions are hashed before being included in the merkle tree, the leaves of the Merkle tree are the hashes, not the transactions themselves.                                                                                                                                                                                | Must  be of length 32                                                                                                                                                                            |
| ValidatorHash     | slice of bytes (`[]byte`) | MerkleRoot of the current validator set. The validators are first sorted by voting power (descending), then by address (ascending) prior to computing the MerkleRoot.                                                                                                                                                                                                                 | Must  be of length 32                                                                                                                                                                            |
| NextValidatorHash | slice of bytes (`[]byte`) | MerkleRoot of the next validator set. The validators are first sorted by voting power (descending), then by address (ascending) prior to computing the MerkleRoot.                                                                                                                                                                                                                    | Must  be of length 32                                                                                                                                                                            |
| ConsensusHash     | slice of bytes (`[]byte`) | Hash of the protobuf encoded consensus parameters.                                                                                                                                                                                                                                                                                                                                    | Must  be of length 32                                                                                                                                                                            |
| AppHash           | slice of bytes (`[]byte`) | Arbitrary byte array returned by the application after executing and commiting the previous block. It serves as the basis for validating any merkle proofs that comes from the ABCI application and represents the state of the actual application rather than the state of the blockchain itself. The first block's `block.Header.AppHash` is given by `ResponseInitChain.app_hash`. | This hash is determined by the application, Tendermint can not perform validation on it.                                                                                                         |
| LastResultHash    | slice of bytes (`[]byte`) | `LastResultsHash` is the root hash of a Merkle tree built from `ResponseDeliverTx` responses (`Log`,`Info`, `Codespace` and `Events` fields are ignored).                                                                                                                                                                                                                             | Must  be of length 32. The first block has `block.Header.ResultsHash == MerkleRoot(nil)`, i.e. the hash of an empty input, for RFC-6962 conformance.                                             |
| EvidenceHash      | slice of bytes (`[]byte`) | MerkleRoot of the evidence of Byzantine behaviour included in this block.                                                                                                                                                                                                                                                                                                             | Must  be of length 32                                                                                                                                                                            |
| ProposerAddress   | slice of bytes (`[]byte`) | Address of the original proposer of the block. Validator must be in the current validatorSet.                                                                                                                                                                                                                                                                                         | Must  be of length 20                                                                                                                                                                            |

## Version

NOTE: that this is more specifically the consensus version and doesn't include information like the
P2P Version. (TODO: we should write a comprehensive document about
versioning that this can refer to)

| Name  | type   | Description                                                                                                     | Validation                                                                                                         |
|-------|--------|-----------------------------------------------------------------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------|
| Block | uint64 | This number represents the version of the block protocol and must be the same throughout an operational network | Must be equal to protocol version being used in a network (`block.Version.Block == state.Version.Consensus.Block`) |
| App   | uint64 | App version is decided on by the application. Read [here](../abci/abci.md#info)                                 | `block.Version.App == state.Version.Consensus.App`                                                                 |

## BlockID

The `BlockID` contains two distinct Merkle roots of the block. The `BlockID` includes these two hashes, as well as the number of parts (ie. `len(MakeParts(block))`)

| Name          | Type                            | Description                                                                                                                                                      | Validation                                                             |
|---------------|---------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------|------------------------------------------------------------------------|
| Hash          | slice of bytes (`[]byte`)       | MerkleRoot of all the fields in the header (ie. `MerkleRoot(header)`.                                                                                            | hash must be of length 32                                              |
| PartSetHeader | [PartSetHeader](#partsetheader) | Used for secure gossiping of the block during consensus, is the MerkleRoot of the complete serialized block cut into parts (ie. `MerkleRoot(MakeParts(block))`). | Must adhere to the validation rules of [PartSetHeader](#partsetheader) |

See [MerkleRoot](./encoding.md#MerkleRoot) for details.

## PartSetHeader

| Name  | Type                      | Description                       | Validation           |
|-------|---------------------------|-----------------------------------|----------------------|
| Total | int32                     | Total amount of parts for a block | Must be > 0          |
| Hash  | slice of bytes (`[]byte`) | MerkleRoot of a serialized block  | Must be of length 32 |

## Part

Part defines a part of a block. In Tendermint blocks are broken into `parts` for gossip.

| Name  | Type            | Description                       | Validation           |
|-------|-----------------|-----------------------------------|----------------------|
| index | int32           | Total amount of parts for a block | Must be > 0          |
| bytes | bytes           | MerkleRoot of a serialized block  | Must be of length 32 |
| proof | [Proof](#proof) | MerkleRoot of a serialized block  | Must be of length 32 |

## Time

Tendermint uses the [Google.Protobuf.Timestamp](https://developers.google.com/protocol-buffers/docs/reference/google.protobuf#google.protobuf.Timestamp)
format, which uses two integers, one 64 bit integer for Seconds and a 32 bit integer for Nanoseconds.

## Data

Data is just a wrapper for a list of transactions, where transactions are arbitrary byte arrays:

| Name | Type                       | Description            | Validation                                                                  |
|------|----------------------------|------------------------|-----------------------------------------------------------------------------|
| Txs  | Matrix of bytes ([][]byte) | Slice of transactions. | Validation does not occur on this field, this data is unknown to Tendermint |

## Commit

Commit is a simple wrapper for a list of signatures, with one for each validator. It also contains the relevant BlockID, height and round:

| Name       | Type                             | Description                                                          | Validation                                                                                               |
|------------|----------------------------------|----------------------------------------------------------------------|----------------------------------------------------------------------------------------------------------|
| Height     | uint64                            | Height at which this commit was created.                             | Must be > 0                                                                                              |
| Round      | int32                            | Round that the commit corresponds to.                                | Must be > 0                                                                                              |
| BlockID    | [BlockID](#blockid)              | The blockID of the corresponding block.                              | Must adhere to the validation rules of [BlockID](#blockid).                                              |
| Signatures | Array of [CommitSig](#commitsig) | Array of commit signatures that correspond to current validator set. | Length of signatures must be > 0 and adhere to the validation of each individual [Commitsig](#commitsig) |

## CommitSig

`CommitSig` represents a signature of a validator, who has voted either for nil,
a particular `BlockID` or was absent. It's a part of the `Commit` and can be used
to reconstruct the vote set given the validator set.

| Name             | Type                        | Description                                                                                                                                                     | Validation                                                        |
|------------------|-----------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------|-------------------------------------------------------------------|
| BlockIDFlag      | [BlockIDFlag](#blockidflag) | Represents the validators participation in consensus: Either voted for the block that received the majority, voted for another block, voted nil or did not vote | Must be one of the fields in the [BlockIDFlag](#blockidflag) enum |
| ValidatorAddress | [Address](#address)         | Address of the validator                                                                                                                                        | Must be of length 20                                              |
| Timestamp        | [Time](#time)               | This field will vary from `CommitSig` to `CommitSig`. It represents the timestamp of the validator.                                                             | [Time](#time)                                                     |
| Signature        | [Signature](#signature)     | Signature corresponding to the validators participation in consensus.                                                                                           | The length of the signature must be > 0 and < than  64            |

NOTE: `ValidatorAddress` and `Timestamp` fields may be removed in the future
(see [ADR-25](https://github.com/tendermint/tendermint/blob/master/docs/architecture/adr-025-commit.md)).

## BlockIDFlag

BlockIDFlag represents which BlockID the [signature](#commitsig) is for.

```go
enum BlockIDFlag {
  BLOCK_ID_FLAG_UNKNOWN = 0;
  BLOCK_ID_FLAG_ABSENT  = 1; // signatures for other blocks are also considered absent
  BLOCK_ID_FLAG_COMMIT  = 2;
  BLOCK_ID_FLAG_NIL     = 3;
}
```

## Vote

A vote is a signed message from a validator for a particular block.
The vote includes information about the validator signing it. When stored in the blockchain or propagated over the network, votes are encoded in Protobuf.
The vote extension is not part of the [`CanonicalVote`](#canonicalvote).

| Name               | Type                            | Description                                                                                 | Validation                                                                                           |
|--------------------|---------------------------------|---------------------------------------------------------------------------------------------|------------------------------------------------------------------------------------------------------|
| Type               | [SignedMsgType](#signedmsgtype) | Either prevote or precommit. [SignedMsgType](#signedmsgtype)                                | A Vote is valid if its corresponding fields are included in the enum [signedMsgType](#signedmsgtype) |
| Height             | uint64                          | Height for which this vote was created.                                                 | Must be > 0                                                                                          |
| Round              | int32                           | Round that the commit corresponds to.                                                       | Must be > 0                                                                                          |
| BlockID            | [BlockID](#blockid)             | The blockID of the corresponding block.                                                     | [BlockID](#blockid)                                                                                  |
| Timestamp          | [Time](#time)                   | The time at which a validator signed.                                  | [Time](#time)                                                                                        |
| ValidatorAddress   | slice of bytes (`[]byte`)       | Address of the validator                                                                    | Length must be equal to 20                                                                           |
| ValidatorIndex     | int32                           | Index at a specific block height that corresponds to the Index of the validator in the set. | must be > 0                                                                                          |
| Signature          | slice of bytes (`[]byte`)       | Signature by the validator if they participated in consensus for the associated bock.       | Length of signature must be > 0 and < 64                                                             |
| Extension          | slice of bytes (`[]byte`)       | The vote extension provided by the Application. Only valid for precommit messages.          | Length must be 0 if Type != `SIGNED_MSG_TYPE_PRECOMMIT`                                              |
| ExtensionSignature | slice of bytes (`[]byte`)       | Signature by the validator if they participated in consensus for the associated bock.       | Length must be 0 if Type != `SIGNED_MSG_TYPE_PRECOMMIT`; else length must be > 0 and < 64            |

## CanonicalVote

CanonicalVote is for validator signing. This type will not be present in a block. Votes are represented via `CanonicalVote` and also encoded using protobuf via `type.SignBytes` which includes the `ChainID`, and uses a different ordering of
the fields.

```proto
message CanonicalVote {
  SignedMsgType             type      = 1;
  fixed64                   height    = 2;
  sfixed64                  round     = 3;
  CanonicalBlockID          block_id  = 4;
  google.protobuf.Timestamp timestamp = 5;
  string                    chain_id  = 6;
}
```

For signing, votes are represented via [`CanonicalVote`](#canonicalvote) and also encoded using protobuf via
`type.SignBytes` which includes the `ChainID`, and uses a different ordering of
the fields.

We define a method `Verify` that returns `true` if the signature verifies against the pubkey for the `SignBytes`
using the given ChainID:

```go
func (vote *Vote) Verify(chainID string, pubKey crypto.PubKey) error {
 if !bytes.Equal(pubKey.Address(), vote.ValidatorAddress) {
  return ErrVoteInvalidValidatorAddress
 }

 if !pubKey.VerifyBytes(types.VoteSignBytes(chainID), vote.Signature) {
  return ErrVoteInvalidSignature
 }
 return nil
}
```

## Proposal

Proposal contains height and round for which this proposal is made, BlockID as a unique identifier
of proposed block, timestamp, and POLRound (a so-called Proof-of-Lock (POL) round) that is needed for
termination of the consensus. If POLRound >= 0, then BlockID corresponds to the block that
is locked in POLRound. The message is signed by the validator private key.

| Name      | Type                            | Description                                                                           | Validation                                              |
|-----------|---------------------------------|---------------------------------------------------------------------------------------|---------------------------------------------------------|
| Type      | [SignedMsgType](#signedmsgtype) | Represents a Proposal [SignedMsgType](#signedmsgtype)                                 | Must be `ProposalType`  [signedMsgType](#signedmsgtype) |
| Height    | uint64                           | Height for which this vote was created for                                            | Must be > 0                                             |
| Round     | int32                           | Round that the commit corresponds to.                                                 | Must be > 0                                             |
| POLRound  | int64                           | Proof of lock                                                                         | Must be > 0                                             |
| BlockID   | [BlockID](#blockid)             | The blockID of the corresponding block.                                               | [BlockID](#blockid)                                     |
| Timestamp | [Time](#time)                   | Timestamp represents the time at which a validator signed.                            | [Time](#time)                                           |
| Signature | slice of bytes (`[]byte`)       | Signature by the validator if they participated in consensus for the associated bock. | Length of signature must be > 0 and < 64                |

## SignedMsgType

Signed message type represents a signed messages in consensus.

```proto
enum SignedMsgType {

  SIGNED_MSG_TYPE_UNKNOWN = 0;
  // Votes
  SIGNED_MSG_TYPE_PREVOTE   = 1;
  SIGNED_MSG_TYPE_PRECOMMIT = 2;

  // Proposal
  SIGNED_MSG_TYPE_PROPOSAL = 32;
}
```

## Signature

Signatures in Tendermint are raw bytes representing the underlying signature.

See the [signature spec](./encoding.md#key-types) for more.

## EvidenceList

EvidenceList is a simple wrapper for a list of evidence:

| Name     | Type                           | Description                            | Validation                                                      |
|----------|--------------------------------|----------------------------------------|-----------------------------------------------------------------|
| Evidence | Array of [Evidence](#evidence) | List of verified [evidence](#evidence) | Validation adheres to individual types of [Evidence](#evidence) |

## Evidence

Evidence in Tendermint is used to indicate breaches in the consensus by a validator.

More information on how evidence works in Tendermint can be found [here](../consensus/evidence.md)

### DuplicateVoteEvidence

`DuplicateVoteEvidence` represents a validator that has voted for two different blocks
in the same round of the same height. Votes are lexicographically sorted on `BlockID`.

| Name             | Type          | Description                                                        | Validation                                          |
|------------------|---------------|--------------------------------------------------------------------|-----------------------------------------------------|
| VoteA            | [Vote](#vote) | One of the votes submitted by a validator when they equivocated    | VoteA must adhere to [Vote](#vote) validation rules |
| VoteB            | [Vote](#vote) | The second vote submitted by a validator when they equivocated     | VoteB must adhere to [Vote](#vote) validation rules |
| TotalVotingPower | int64         | The total power of the validator set at the height of equivocation | Must be equal to nodes own copy of the data         |
| ValidatorPower   | int64         | Power of the equivocating validator at the height                  | Must be equal to the nodes own copy of the data     |
| Timestamp        | [Time](#time) | Time of the block where the equivocation occurred                  | Must be equal to the nodes own copy of the data     |

### LightClientAttackEvidence

`LightClientAttackEvidence` is a generalized evidence that captures all forms of known attacks on
a light client such that a full node can verify, propose and commit the evidence on-chain for
punishment of the malicious validators. There are three forms of attacks: Lunatic, Equivocation
and Amnesia. These attacks are exhaustive. You can find a more detailed overview of this [here](../light-client/accountability#the_misbehavior_of_faulty_validators)

| Name                 | Type                             | Description                                                          | Validation                                                       |
|----------------------|----------------------------------|----------------------------------------------------------------------|------------------------------------------------------------------|
| ConflictingBlock     | [LightBlock](#lightblock)        | Read Below                                                           | Must adhere to the validation rules of [lightBlock](#lightblock) |
| CommonHeight         | int64                            | Read Below                                                           | must be > 0                                                      |
| Byzantine Validators | Array of [Validator](#validator) | validators that acted maliciously                                    | Read Below                                                       |
| TotalVotingPower     | int64                            | The total power of the validator set at the height of the infraction | Must be equal to the nodes own copy of the data                  |
| Timestamp            | [Time](#time)                    | Time of the block where the infraction occurred                      | Must be equal to the nodes own copy of the data                  |

## LightBlock

LightBlock is the core data structure of the [light client](../light-client/README.md). It combines two data structures needed for verification ([signedHeader](#signedheader) & [validatorSet](#validatorset)).

| Name         | Type                          | Description                                                                                                                            | Validation                                                                          |
|--------------|-------------------------------|----------------------------------------------------------------------------------------------------------------------------------------|-------------------------------------------------------------------------------------|
| SignedHeader | [SignedHeader](#signedheader) | The header and commit, these are used for verification purposes. To find out more visit [light client docs](../light-client/README.md) | Must not be nil and adhere to the validation rules of [signedHeader](#signedheader) |
| ValidatorSet | [ValidatorSet](#validatorset) | The validatorSet is used to help with verify that the validators in that committed the infraction were truly in the validator set.     | Must not be nil and adhere to the validation rules of [validatorSet](#validatorset) |

The `SignedHeader` and `ValidatorSet` are linked by the hash of the validator set(`SignedHeader.ValidatorsHash == ValidatorSet.Hash()`.

## SignedHeader

The SignedhHeader is the [header](#header) accompanied by the commit to prove it.

| Name   | Type              | Description       | Validation                                                                        |
|--------|-------------------|-------------------|-----------------------------------------------------------------------------------|
| Header | [Header](#header) | [Header](#header) | Header cannot be nil and must adhere to the [Header](#header) validation criteria |
| Commit | [Commit](#commit) | [Commit](#commit) | Commit cannot be nil and must adhere to the [Commit](#commit) criteria            |

## ValidatorSet

| Name       | Type                             | Description                                        | Validation                                                                                                        |
|------------|----------------------------------|----------------------------------------------------|-------------------------------------------------------------------------------------------------------------------|
| Validators | Array of [validator](#validator) | List of the active validators at a specific height | The list of validators can not be empty or nil and must adhere to the validation rules of [validator](#validator) |
| Proposer   | [validator](#validator)          | The block proposer for the corresponding block     | The proposer cannot be nil and must adhere to the validation rules of  [validator](#validator)                    |

## Validator

| Name             | Type                      | Description                                                                                       | Validation                                        |
|------------------|---------------------------|---------------------------------------------------------------------------------------------------|---------------------------------------------------|
| Address          | [Address](#address)       | Validators Address                                                                                | Length must be of size 20                         |
| Pubkey           | slice of bytes (`[]byte`) | Validators Public Key                                                                             | must be a length greater than 0                   |
| VotingPower      | int64                     | Validators voting power                                                                           | cannot be < 0                                     |
| ProposerPriority | int64                     | Validators proposer priority. This is used to gauge when a validator is up next to propose blocks | No validation, value can be negative and positive |

## Address

Address is a type alias of a slice of bytes. The address is calculated by hashing the public key using sha256 and truncating it to only use the first 20 bytes of the slice.

```go
const (
  TruncatedSize = 20
)

func SumTruncated(bz []byte) []byte {
  hash := sha256.Sum256(bz)
  return hash[:TruncatedSize]
}
```

## ConsensusParams

| Name      | Type                                | Description                                                                  | Field Number |
|-----------|-------------------------------------|------------------------------------------------------------------------------|--------------|
| block     | [BlockParams](#blockparams)         | Parameters limiting the size of a block and time between consecutive blocks. | 1            |
| evidence  | [EvidenceParams](#evidenceparams)   | Parameters limiting the validity of evidence of byzantine behaviour.         | 2            |
| validator | [ValidatorParams](#validatorparams) | Parameters limiting the types of public keys validators can use.             | 3            |
| version   | [BlockParams](#blockparams)         | The ABCI application version.                                                | 4            |

### BlockParams

| Name         | Type  | Description                                                                                                                                                                                                 | Field Number |
|--------------|-------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|--------------|
| max_bytes    | int64 | Max size of a block, in bytes.                                                                                                                                                                              | 1            |
| max_gas      | int64 | Max sum of `GasWanted` in a proposed block. NOTE: blocks that violate this may be committed if there are Byzantine proposers. It's the application's responsibility to handle this when processing a block! | 2            |
| recheck_tx   | bool  | Indicated whether to run `CheckTx` on all remaining transactions *after* every execution of a block | 3            |

### EvidenceParams

| Name               | Type                                                                                                                               | Description                                                                                                                                                                                                                                                                    | Field Number |
|--------------------|------------------------------------------------------------------------------------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|--------------|
| max_age_num_blocks | int64                                                                                                                              | Max age of evidence, in blocks.                                                                                                                                                                                                                                                | 1            |
| max_age_duration   | [google.protobuf.Duration](https://developers.google.com/protocol-buffers/docs/reference/google.protobuf#google.protobuf.Duration) | Max age of evidence, in time. It should correspond with an app's "unbonding period" or other similar mechanism for handling [Nothing-At-Stake attacks](https://github.com/ethereum/wiki/wiki/Proof-of-Stake-FAQ#what-is-the-nothing-at-stake-problem-and-how-can-it-be-fixed). | 2            |
| max_bytes          | int64                                                                                                                              | maximum size in bytes of total evidence allowed to be entered into a block                                                                                                                                                                                                     | 3            |

### ValidatorParams

| Name          | Type            | Description                                                           | Field Number |
|---------------|-----------------|-----------------------------------------------------------------------|--------------|
| pub_key_types | repeated string | List of accepted public key types. Uses same naming as `PubKey.Type`. | 1            |

### VersionParams

| Name        | Type   | Description                   | Field Number |
|-------------|--------|-------------------------------|--------------|
| app_version | uint64 | The ABCI application version. | 1            |

### SynchronyParams

| Name          | Type   | Description                   | Field Number |
|---------------|--------|-------------------------------|--------------|
| message_delay | [google.protobuf.Duration](https://developers.google.com/protocol-buffers/docs/reference/google.protobuf#google.protobuf.Duration) | Bound for how long a proposal message may take to reach all validators on a newtork and still be considered valid. | 1            |
| precision     | [google.protobuf.Duration](https://developers.google.com/protocol-buffers/docs/reference/google.protobuf#google.protobuf.Duration) | Bound for how skewed a proposer's clock may be from any validator on the network while still producing valid proposals. | 2            |

### TimeoutParams

| Name          | Type   | Description                   | Field Number |
|---------------|--------|-------------------------------|--------------|
| propose | [google.protobuf.Duration](https://developers.google.com/protocol-buffers/docs/reference/google.protobuf#google.protobuf.Duration) | Parameter that, along with propose_delta, configures the timeout for the propose step of the consensus algorithm. | 1 |
| propose_delta | [google.protobuf.Duration](https://developers.google.com/protocol-buffers/docs/reference/google.protobuf#google.protobuf.Duration) | Parameter that, along with propose, configures the timeout for the propose step of the consensus algorithm. | 2 |
| vote | [google.protobuf.Duration](https://developers.google.com/protocol-buffers/docs/reference/google.protobuf#google.protobuf.Duration)| Parameter that, along with vote_delta, configures the timeout for the prevote and precommit step of the consensus algorithm. | 3 |
| vote_delta | [google.protobuf.Duration](https://developers.google.com/protocol-buffers/docs/reference/google.protobuf#google.protobuf.Duration)| Parameter that, along with vote, configures the timeout for the prevote and precommit step of the consensus algorithm. | 4 |
| commit | [google.protobuf.Duration](https://developers.google.com/protocol-buffers/docs/reference/google.protobuf#google.protobuf.Duration) | Parameter that configures how long Tendermint will wait after receiving a quorum of precommits before beginning consensus for the next height.| 5 |
| bypass_commit_timeout | bool | Parameter that, if enabled, configures the node to proceed immediately to the next height once the node has received all precommits for a block, forgoing the commit timeout. |  6  |

## Proof

| Name      | Type           | Description                                   | Field Number |
|-----------|----------------|-----------------------------------------------|--------------|
| total     | int64          | Total number of items.                        | 1            |
| index     | int64          | Index item to prove.                          | 2            |
| leaf_hash | bytes          | Hash of item value.                           | 3            |
| aunts     | repeated bytes | Hashes from leaf's sibling to a root's child. | 4            |
