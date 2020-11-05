# Data Structures

Here we describe the data structures in the Tendermint blockchain and the rules for validating them.

The Tendermint blockchains consists of a short list of data types:

- [`Block`](#block)
- [`Header`](#header)
- [`Version`](#version)
- [`BlockID`](#blockid)
- [`PartSetHeader`](#partsetheader)
- [`Time`](#time)
- [`Data` (for transactions)](#data)
- [`Commit`](#commit)
- [`CommitSig`](#commitsig)
- [`BlockIDFlag`](#blockidflag)
- [`Vote`](#vote)
- [`CanonicalVote`](#canonicalvote)
- [`SignedMsgType`](#signedmsgtype)
- [`EvidenceData`](#evidence_data)
- [`Evidence`](#evidence)
- [`DuplicateVoteEvidence`](#duplicatevoteevidence)
- [`LightClientAttackEvidence`](#lightclientattackevidence)
- [`LightBlock](#lightblock)
- [`SignedHeader`](#signedheader)
- [`Validator`](#validator)
- [`ValidatorSet`](#validatorset)
- [`Address`](#address)

## Block

A block consists of a header, transactions, votes (the commit),
and a list of evidence of malfeasance (ie. signing conflicting votes).

| Name       | Type                           | Description                                                                                                                                                                          | Validation                                                                                       |
|------------|--------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|--------------------------------------------------------------------------------------------------|
| Header     | [Header](#header)              | Header corresponding to the block. This field contains information used throughout consensus and other areas of the protocol. To find out what it contains, visit [header] (#header) | Must adhere to the validation rules of [header](#header)                                         |
| Data       | [Data](#data)                  | Data contains a list of transactions. The contents of the transaction is unknown to Tendermint.                                                                                      | This field can be empty or populated, but no validation is performed. Applications can perform validation on individual transactions prior to block creation using [checkTx](../abci/abci.md#checktx).
| Evidence   | [EvidenceData](#evidence_data) | Evidence contains a list of infractions committed by validators.                                                                                                                     | Can be empty, but when populated the validations rules from [evidenceData](#evidence_data) apply |
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

| Name              | Type                | Description                                                                                                                                                                                                                                                                                                                                                                           | Validation                                                                                                                                                                                                             |
|-------------------|---------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Version           | [Version](#version) | Version defines the application and protocol verion being used.                                                                                                                                                                                                                                                                                                                       | Must adhere to the validation rules of [Version](#version)                                                                                                                                                                                                    |
| ChainID           | String              | ChainID is the ID of the chain. This must be unique to your chain.                                                                                                                                                                                                                                                                                                                    | ChainID must be less than 50 bytes.                                                                                                                                                                                    |
| Height            | int64               | Height is the height for this header.                                                                                                                                                                                                                                                                                                                                                 | Must be > 0, >= initialHeight, and == previous Height+1                                                                                                                                                                |
| Time              | [Time](#time)       |                   The timestamp is equal to the weighted median of validators present in the last commit. Read more on time in the [BFT-time section](../consensus/bft-time.md). Note: the timestamp of a vote must be greater by at least one millisecond than that of the block being voted on.                                                                                                                                                                                                                                                                                                                                                                    | Time must be >= previous header timestamp + consensus parameters TimeIotaMs.  The timestamp of the first block must be equal to the genesis time (since there's no votes to compute the median).  |
| LastBlockID       | [BlockID](#blockid) |               BlockID of the previous block.                                                                                                                                                                                                                                                                                                                                                                        | Must adhere to the validation rules of [blockID](#blockid). The first block has `block.Header.LastBlockID == BlockID{}`.                                                                                                                                                                                                    |
| LastCommitHash    | slice of bytes (`[]byte`)     | MerkleRoot of the lastCommit's signatures. The signatures represent the validators that committed to the last block. The first block has an empty slices of bytes for the hash.                                                                                                                                                                                                       | Must  be of length 32                                                                                                                                                                                                  |
| DataHash          | slice of bytes (`[]byte`)     | MerkleRoot of the hash of transactions. **Note**: The transactions are hashed before being included in the merkle tree, the leaves of the Merkle tree are the hashes, not the transactions themselves.                                                                                                                                                                                | Must  be of length 32                                                                                                                                                                                                  |
| ValidatorHash     | slice of bytes (`[]byte`)     | MerkleRoot of the current validator set. The validators are first sorted by voting power (descending), then by address (ascending) prior to computing the MerkleRoot.                                                                                                                                                                                                                 | Must  be of length 32                                                                                                                                                                                                  |
| NextValidatorHash | slice of bytes (`[]byte`)     | MerkleRoot of the next validator set. The validators are first sorted by voting power (descending), then by address (ascending) prior to computing the MerkleRoot.                                                                                                                                                                                                                    | Must  be of length 32                                                                                                                                                                                                  |
| ConsensusHash     | slice of bytes (`[]byte`)     | Hash of the protobuf encoded consensus parameters.                                                                                                                                                                                                                                                                                                                               | Must  be of length 32                                                                                                                                                                                                  |
| AppHash           | slice of bytes (`[]byte`)     | Arbitrary byte array returned by the application after executing and commiting the previous block. It serves as the basis for validating any merkle proofs that comes from the ABCI application and represents the state of the actual application rather than the state of the blockchain itself. The first block's `block.Header.AppHash` is given by `ResponseInitChain.app_hash`. | This hash is determined by the application, Tendermint can not perform validation on it.                                                                                                                              |
| LastResultHash    | slice of bytes (`[]byte`)     | `LastResultsHash` is the root hash of a Merkle tree built from `ResponseDeliverTx` responses (`Log`,`Info`, `Codespace` and `Events` fields are ignored).                                                                | Must  be of length 32. The first block has `block.Header.ResultsHash == MerkleRoot(nil)`, i.e. the hash of an empty input, for RFC-6962 conformance.                                                                                                                                                                                                                            |
| EvidenceHash      | slice of bytes (`[]byte`)      | MerkleRoot of the evidence of Byzantine behaviour included in this block.                                                                                                                                                                                                                                                                                                             | Must  be of length 32                                                                                                                                                                                                  |
| ProposerAddress   | slice of bytes (`[]byte`)      | Address of the original proposer of the block. Validator must be in the current validatorSet.                                                                                                                                                                                                                                                                                                           | Must  be of length 20                                                                                                                                                                                                  |

## Version

| Name  | type   | Description | Validation                                                                                                       |
|-------|--------|-------------|------------------------------------------------------------------------------------------------------------------|
| Block | uint64 |     This number represents the version of the block protocol and must be the same throughout an operational network         | Must be equal to protocol version being used in a network (`block.Version.Block == state.Version.Consensus.Block`) |
| App   | uint64 |  App version is decided on by the application. Read [here](../abci/abci.md#info)         | `block.Version.App == state.Version.Consensus.App`                                                               |

## BlockID

The `BlockID` contains two distinct Merkle roots of the block. The `BlockID` includes these two hashes, as well as the number of parts (ie. `len(MakeParts(block))`)

| Name        | Type                        | Description                                                                                                                                                      | Validation                                                         |
|-------------|-----------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------|--------------------------------------------------------------------|
| Hash        | slice of bytes (`[]byte`)   | MerkleRoot of all the fields in the header (ie. `MerkleRoot(header)`.                                                                                            | hash must be of length 32                                          |
| PartSetHeader | [PartSetHeader](#PartSetHeader) | Used for secure gossiping of the block during consensus, is the MerkleRoot of the complete serialized block cut into parts (ie. `MerkleRoot(MakeParts(block))`). | Must adhere to the validation rules of [PartSetHeader](#PartSetHeader) |

See [MerkleRoot](./encoding.md#MerkleRoot) for details.

## PartSetHeader

| Name  | Type                      | Description                       | Validation           |
|-------|---------------------------|-----------------------------------|----------------------|
| Total | int32                     | Total amount of parts for a block | Must be > 0          |
| Hash  | slice of bytes (`[]byte`) | MerkleRoot of a serialized block  | Must be of length 32 |

## Time

Tendermint uses the [Google.Protobuf.WellKnownTypes.Timestamp](https://developers.google.com/protocol-buffers/docs/reference/csharp/class/google/protobuf/well-known-types/timestamp)
format, which uses two integers, one for Seconds and for Nanoseconds.

## Data

Data is just a wrapper for a list of transactions, where transactions are arbitrary byte arrays:

| Name | Type                       | Description            | Validation                                                                  |
|------|----------------------------|------------------------|-----------------------------------------------------------------------------|
| Txs  | Matrix of bytes ([][]byte) | Slice of transactions. | Validation does not occur on this field, this data is unknown to Tendermint |

## Commit

Commit is a simple wrapper for a list of signatures, with one for each validator. It also contains the relevant BlockID, height and round:

| Name       | Type                             | Description                                                          | Validation                                                                                               |
|------------|----------------------------------|----------------------------------------------------------------------|----------------------------------------------------------------------------------------------------------|
| Height     | int64                            | Height at which this commit was created.                              | Must be > 0                                                                                              |
| Round      | int32                            | Round that the commit corresponds to.                                | Must be > 0                                                                                              |
| BlockID    | [BlockID](#blockid)              | The blockID of the corresponding block.                              | Must adhere to the validation rules of [BlockID](#blockid).                                              |
| Signatures | Array of [CommitSig](#commitsig) | Array of commit signatures that correspond to current validator set. | Length of signatures must be > 0 and adhere to the validation of each individual [Commitsig](#commitsig) |

## CommitSig

`CommitSig` represents a signature of a validator, who has voted either for nil,
a particular `BlockID` or was absent. It's a part of the `Commit` and can be used
to reconstruct the vote set given the validator set.

| Name             | Type                        | Description                                                                                                  | Validation                                                        |
|------------------|-----------------------------|---------------------------------------------------------------------------------------------------------------|-------------------------------------------------------------------|
| BlockIDFlag      | [BlockIDFlag](#blockidflag) | Represents the validators participation in consensus: Either voted for the block that received the majority, voted for another block, voted nil or did not vote | Must be one of the fields in the [BlockIDFlag](#blockidflag) enum |
| ValidatorAddress | [Address](#address)         | Address of the validator                                                                                      | Must be of length 20                                              |
| Timestamp        | [Time](#time)               | This field will vary from `CommitSig` to `CommitSig`. It represents the timestamp of the validator.               | [Time](#time)                                                     |
| Signature        | [Signature](#signature)     | Signature corresponding to the validators participation in consensus.                                         | The length of the signature must be > 0 and < than  64            |

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

| Name             | Type                            | Description                                                                                 | Validation                                                                                           |
|------------------|---------------------------------|---------------------------------------------------------------------------------------------|------------------------------------------------------------------------------------------------------|
| Type             | [SignedMsgType](#signedmsgtype) | Either prevote or precommit. [SignedMsgType](#signedmsgtype)                                                             | A Vote is valid if its corresponding fields are included in the enum [signedMsgType](#signedmsgtype) |
| Height           | int64                           | Height for which this vote was created for                                                  | Must be > 0                                                                                          |
| Round            | int32                           | Round that the commit corresponds to.                                                       | Must be > 0                                                                                          |
| BlockID          | [BlockID](#blockid)             | The blockID of the corresponding block.                                                     | [BlockID](#blockid)                                                                                  |
| Timestamp        | [Time](#Time)                   | Timestamp represents the time at which a validator signed.                                  | [Time](#time)                                                                                        |
| ValidatorAddress | slice of bytes (`[]byte`)       | Address of the validator                                                                    | Length must be equal to 20                                                                           |
| ValidatorIndex   | int32                           | Index at a specific block height that corresponds to the Index of the validator in the set. | must be > 0                                                                                          |
| Signature        | slice of bytes (`[]byte`)       | Signature by the validator if they participated in consensus for the associated bock.       | Length of signature must be > 0 and < 64                                                             |

## CanonicalVote

CanonicalVote is for validator signing. This type will not be present in a block. Votes are represented via `CanonicalVote` and also encoded using protobuf via `type.SignBytes` which includes the `ChainID`, and uses a different ordering of
the fields.

```proto
message CanonicalVote {
  SignedMsgType             type      = 1;
  sfixed64                  height    = 2;
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

## EvidenceData

EvidenceData is a simple wrapper for a list of evidence:

| Name     | Type                           | Description                              | Validation                                                      |
|----------|--------------------------------|------------------------------------------|-----------------------------------------------------------------|
| Evidence | Array of [Evidence](#evidence) | List of verified [evidence](#evidence) | Validation adheres to individual types of [Evidence](#evidence) |

## Evidence

Evidence in Tendermint is used to indicate breaches in the consensus by a validator.

The [Fork Accountability](../light-client/accountability.md) document provides a good overview of evidence and how they occur. For evidence to be committed onchain, it must adhere to the validation rules of each evidence and must not be expired. The expiration age, measured in both block height and time is set in `EvidenceParams`. Each evidence uses
the timestamp of the block that the evidence occurred at to indicate the age of the evidence.

### DuplicateVoteEvidence

`DuplicateVoteEvidence` represents a validator that has voted for two different blocks
in the same round of the same height. Votes are lexicographically sorted on `BlockID`.

| Name  | Type          | Description                                                     | Validation                                          |
|-------|---------------|-----------------------------------------------------------------|-----------------------------------------------------|
| VoteA | [Vote](#vote) | One of the votes submitted by a validator when they equivocated | VoteA must adhere to [Vote](#vote) validation rules |
| VoteB | [Vote](#vote) | The second vote submitted by a validator when they equivocated  | VoteB must adhere to [Vote](#vote) validation rules |

Valid Duplicate Vote Evidence must adhere to the following rules:

- Validator Address, Height, Round and Type must be the same for both votes

- BlockID must be different for both votes (BlockID can be for a nil block)

- Validator must have been in the validator set at that height

- Vote signature must be valid (using the chainID)

- For DuplicateVoteEvidence to be included in a block it must be within the time period outlined in [evidenceParams](../abci/abci.md#evidenceparams). Evidence expiration is measured as a combination of age in terms of height and time.

### LightClientAttackEvidence

LightClientAttackEvidence is a generalized evidence that captures all forms of known attacks on
a light client such that a full node can verify, propose and commit the evidence on-chain for
punishment of the malicious validators. There are three forms of attacks: Lunatic, Equivocation
and Amnesia. These attacks are exhaustive. You can find a more detailed overview of this [here](../light-client/accountability#the_misbehavior_of_faulty_validators)

| Name             | Type                      | Description | Validation |
|------------------|---------------------------|-------------|------------|
| ConflictingBlock | [LightBlock](#LightBlock) | Read Below  | Must adhere to the validation rules of [lightBlock](#lightblock) |
| CommonHeight     | int64                     | Read Below  | must be > 0 |

Valid Light Client Attack Evidence must adhere to the following rules:

- If the header of the light block is invalid, thus indicating a lunatic attack, the node must check that
    they can use `verifySkipping` from their header at the common height to the conflicting header

- If the header is valid, then the validator sets are the same and this is either a form of equivocation
    or amnesia. We therefore check that 2/3 of the validator set also signed the conflicting header

- The trusted header of the node at the same height as the conflicting header must have a different hash to
    the conflicting header.

- For LightClientAttackEvidence to be included in a block it must be within the time period outlined in [evidenceParams](../abci/abci.md#evidenceparams). Evidence expiration is measured as a combination of age in terms of height and time.

## LightBlock

LightBlock is the core data structure of the [light client](../light-client/README.md). It combines two data structures needed for verification ([signedHeader](#signedheader) & [validatorSet](#validatorset)).

| Name         | Type                          | Description                                                                                                                            | Validation                                                                          |
|--------------|-------------------------------|----------------------------------------------------------------------------------------------------------------------------------------|-------------------------------------------------------------------------------------|
| SignedHeader | [SignedHeader](#signedheader) | The header and commit, these are used for verification purposes. To find out more visit [light client docs](../light-client/README.md) | Must not be nil and adhere to the validation rules of [signedHeader](#signedheader) |
| ValidatorSet | [ValidatorSet](#validatorset) | The validatorSet is used to help with verify that the validators in that committed the infraction were truly in the validator set.     | Must not be nil and adhere to the validation rules of [validatorSet](#validatorset) |

The `SignedHeader` and `ValidatorSet` are linked by the hash of the validator set(`SignedHeader.ValidatorsHash == ValidatorSet.Hash()`.

## SignedHeader

The SignedhHeader is the [header](#header) accompanied by the commit to prove it.

| Name   | Type              | Description       | Validation                                                                      |
|--------|-------------------|-------------------|---------------------------------------------------------------------------------|
| Header | [Header](#Header) | [Header](#header) | Header cannot be nil and must adhere to the [Header](#Header) validation criteria |
| Commit | [Commit](#commit) | [Commit](#commit) | Commit cannot be nil and must adhere to the [Commit](#commit) criteria            |

## ValidatorSet

| Name       | Type                             | Description                                        | Validation                                                                                                        |
|------------|----------------------------------|----------------------------------------------------|-------------------------------------------------------------------------------------------------------------------|
| Validators | Array of [validator](#validator) | List of the active validators at a specific height | The list of validators can not be empty or nil and must adhere to the validation rules of [validator](#validator) |
| Proposer   | [validator](#validator) | The block proposer for the corresponding block           | The proposer cannot be nil and must adhere to the validation rules of  [validator](#validator)                    |

## Validator

| Name             | Type                      | Description                                                                                       | Validation                      |
|------------------|---------------------------|---------------------------------------------------------------------------------------------------|---------------------------------|
| Address          | [Address](#address)       | Validators Address                                                                                | Length must be of size 20       |
| Pubkey           | slice of bytes (`[]byte`) | Validators Public Key                                                                             | must be a length greater than 0 |
| VotingPower      | int64                     | Validators voting power                                                                           | cannot be < 0                   |
| ProposerPriority | int64                     | Validators proposer priority. This is used to gauge when a validator is up next to propose blocks |  No validation, value can be negative and positive                               |

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
