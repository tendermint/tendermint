# Data Structures

Here we describe the data structures in the Tendermint blockchain and the rules for validating them.

The Tendermint blockchains consists of a short list of basic data types:

- `Block`
- `Header`
- `Version`
- `BlockID`
- `Time`
- `Data` (for transactions)
- `Commit` and `Vote`
- `EvidenceData` and `Evidence`

## Block

A block consists of a header, transactions, votes (the commit),
and a list of evidence of malfeasance (ie. signing conflicting votes).

```go
type Block struct {
    Header      Header
    Txs         Data
    Evidence    EvidenceData
    LastCommit  Commit
}
```

Note the `LastCommit` is the set of signatures of validators that committed the last block.

## Header

A block header contains metadata about the block and about the consensus, as well as commitments to
the data in the current block, the previous block, and the results returned by the application:

```go
type Header struct {
 // basic block info
 Version  Version
 ChainID  string
 Height   int64
 Time     Time

 // prev block info
 LastBlockID BlockID

 // hashes of block data
 LastCommitHash []byte // commit from validators from the last block
 DataHash       []byte // MerkleRoot of transaction hashes

 // hashes from the app output from the prev block
 ValidatorsHash     []byte // validators for the current block
 NextValidatorsHash []byte // validators for the next block
 ConsensusHash      []byte // consensus params for current block
 AppHash            []byte // state after txs from the previous block
 LastResultsHash    []byte // root hash of all results from the txs from the previous block

 // consensus info
 EvidenceHash    []byte // evidence included in the block
 ProposerAddress []byte // original proposer of the block
```

Further details on each of these fields is described below.

## Version

```go
type Version struct {
 Block uint64
 App   uint64
}
```

The `Version` contains the protocol version for the blockchain and the
application as two `uint64` values.

## BlockID

The `BlockID` contains two distinct Merkle roots of the block.
The first, used as the block's main hash, is the MerkleRoot
of all the fields in the header (ie. `MerkleRoot(header)`.
The second, used for secure gossipping of the block during consensus,
is the MerkleRoot of the complete serialized block
cut into parts (ie. `MerkleRoot(MakeParts(block))`).
The `BlockID` includes these two hashes, as well as the number of
parts (ie. `len(MakeParts(block))`)

```go
type BlockID struct {
    Hash []byte
    PartSetHeader PartSetHeader
}

type PartSetHeader struct {
    Total uint32
    Hash []byte
}
```

See [MerkleRoot](./encoding.md#MerkleRoot) for details.

## Time

Tendermint uses the
[Google.Protobuf.WellKnownTypes.Timestamp](https://developers.google.com/protocol-buffers/docs/reference/csharp/class/google/protobuf/well-known-types/timestamp)
format, which uses two integers, one for Seconds and for Nanoseconds.

## Data

Data is just a wrapper for a list of transactions, where transactions are
arbitrary byte arrays:

```go
type Data struct {
    Txs [][]byte
}
```

## Commit

Commit is a simple wrapper for a list of signatures, with one for each
validator. It also contains the relevant BlockID, height and round:

```go
type Commit struct {
 Height     int64
 Round      int32
 BlockID    BlockID
 Signatures []CommitSig
}
```

## CommitSig

`CommitSig` represents a signature of a validator, who has voted either for nil,
a particular `BlockID` or was absent. It's a part of the `Commit` and can be used
to reconstruct the vote set given the validator set.

```go
type BlockIDFlag byte

const (
 // BlockIDFlagAbsent - no vote was received from a validator.
 BlockIDFlagAbsent BlockIDFlag = 0x01
 // BlockIDFlagCommit - voted for the Commit.BlockID.
 BlockIDFlagCommit = 0x02
 // BlockIDFlagNil - voted for nil.
 BlockIDFlagNil = 0x03
)

type CommitSig struct {
 BlockIDFlag      BlockIDFlag
 ValidatorAddress Address
 Timestamp        time.Time
 Signature        []byte
}
```

NOTE: `ValidatorAddress` and `Timestamp` fields may be removed in the future
(see
[ADR-25](https://github.com/tendermint/tendermint/blob/master/docs/architecture/adr-025-commit.md)).

## Vote

A vote is a signed message from a validator for a particular block.
The vote includes information about the validator signing it.

```go
type Vote struct {
 Type             SignedMsgType
 Height           int64
 Round            int32
 BlockID          BlockID
 Timestamp        Time
 ValidatorAddress []byte
 ValidatorIndex   int32
 Signature        []byte
}
```

```protobuf
enum SignedMsgType {
  SIGNED_MSG_TYPE_UNKNOWN = 0;
  // Votes
  PREVOTE_TYPE   = 1;
  PRECOMMIT_TYPE = 2;
  // Proposals
  PROPOSAL_TYPE = 32;
}
```

There are two types of votes:
a _prevote_ has `vote.Type == 1` and
a _precommit_ has `vote.Type == 2`.

## Signature

Signatures in Tendermint are raw bytes representing the underlying signature.

See the [signature spec](./encoding.md#key-types) for more.

## EvidenceData

EvidenceData is a simple wrapper for a list of evidence:

```go
type EvidenceData struct {
    Evidence []Evidence
}
```

## Evidence

Evidence in Tendermint is used to indicate breaches in the consensus by a validator.
It is implemented as the following interface.

```go
type Evidence interface {
 Height() int64                                     // height of the equivocation
 Bytes() []byte                                     // bytes which comprise the evidence
 Hash() []byte                                      // hash of the evidence (this is also used for equality)
 ValidateBasic() error                              // consistency check of the data
 String() string                                    // string representation of the evidence
}
```

All evidence can be encoded and decoded to and from Protobuf with the `EvidenceToProto()`
and `EvidenceFromProto()` functions. The [Fork Accountability](../consensus/light-client/accountability.md)
document provides a good overview for the types of evidence and how they occur. For evidence to be committed onchain, it must adhere to the validation rules of each evidence and must not be expired. The expiration age, measured in both block height and time is set in `EvidenceParams`. Each evidence uses
the timestamp of the block that the evidence occured at to indicate the age of the evidence.

### DuplicateVoteEvidence

`DuplicateVoteEvidence` represents a validator that has voted for two different blocks
in the same round of the same height. Votes are lexicographically sorted on `BlockID`.

```go
type DuplicateVoteEvidence struct {
    VoteA  *Vote
    VoteB  *Vote
}
```

Valid Duplicate Vote Evidence must adhere to the following rules:

- Validator Address, Height, Round and Type must be the same for both votes

- BlockID must be different for both votes (BlockID can be for a nil block)

- Validator must have been in the validator set at that height

- Vote signature must be valid (using the chainID)

- Evidence must not have expired: either age in terms of height or time must be
    less than the age stated in the consensus params. Time is the block time that the
    votes were a part of.

### LightClientAttackEvidence

```go
type LightClientAttackEvidence struct {
 ConflictingBlock *LightBlock
 CommonHeight     int64
}
```

Valid Light Client Attack Evidence encompasses three types of attack and must adhere to the following rules

- If the header of the light block is invalid, thus indicating a lunatic attack, the node must check that
    they can use `verifySkipping` from their header at the common height to the conflicting header

- If the header is valid, then the validator sets are the same and this is either a form of equivocation
    or amnesia. We therefore check that 2/3 of the validator set also signed the conflicting header

- The trusted header of the node at the same height as the conflicting header must have a different hash to
    the conflicting header.

- Evidence must not have expired. The height (and thus the time) is taken from the common height.

## Validation

Here we describe the validation rules for every element in a block.
Blocks which do not satisfy these rules are considered invalid.

We abuse notation by using something that looks like Go, supplemented with English.
A statement such as `x == y` is an assertion - if it fails, the item is invalid.

We refer to certain globally available objects:
`block` is the block under consideration,
`prevBlock` is the `block` at the previous height,
and `state` keeps track of the validator set, the consensus parameters
and other results from the application. At the point when `block` is the block under consideration,
the current version of the `state` corresponds to the state
after executing transactions from the `prevBlock`.
Elements of an object are accessed as expected,
ie. `block.Header`.
See the [definition of `State`](./state.md).

### Header

A Header is valid if its corresponding fields are valid.

### Version

```go
block.Version.Block == state.Version.Consensus.Block
block.Version.App == state.Version.Consensus.App
```

The block version must match consensus version from the state.

### ChainID

```go
len(block.ChainID) < 50
```

ChainID must be less than 50 bytes.

### Height

```go
block.Header.Height > 0
block.Header.Height >= state.InitialHeight
block.Header.Height == prevBlock.Header.Height + 1
```

The height is an incrementing integer. The first block has `block.Header.Height == state.InitialHeight`, derived from the genesis file.

### Time

```go
block.Header.Timestamp >= prevBlock.Header.Timestamp + state.consensusParams.Block.TimeIotaMs
block.Header.Timestamp == MedianTime(block.LastCommit, state.LastValidators)
```

The block timestamp must be monotonic.
It must equal the weighted median of the timestamps of the valid signatures in the block.LastCommit.

Note: the timestamp of a vote must be greater by at least one millisecond than that of the
block being voted on.

The timestamp of the first block must be equal to the genesis time (since
there's no votes to compute the median).

```go
if block.Header.Height == state.InitialHeight {
    block.Header.Timestamp == genesisTime
}
```

See the section on [BFT time](../consensus/bft-time.md) for more details.

### LastBlockID

LastBlockID is the previous block's BlockID:

```go
prevBlockParts := MakeParts(prevBlock)
block.Header.LastBlockID == BlockID {
    Hash: MerkleRoot(prevBlock.Header),
    PartsHeader{
        Hash: MerkleRoot(prevBlockParts),
        Total: len(prevBlockParts),
    },
}
```

The first block has `block.Header.LastBlockID == BlockID{}`.

### LastCommitHash

```go
block.Header.LastCommitHash == MerkleRoot(block.LastCommit.Signatures)
```

MerkleRoot of the signatures included in the block.
These are the commit signatures of the validators that committed the previous
block.

The first block has `block.Header.LastCommitHash == []byte{}`

### DataHash

```go
block.Header.DataHash == MerkleRoot(Hashes(block.Txs.Txs))
```

MerkleRoot of the hashes of transactions included in the block.

Note the transactions are hashed before being included in the Merkle tree,
so the leaves of the Merkle tree are the hashes, not the transactions
themselves. This is because transaction hashes are regularly used as identifiers for
transactions.

### ValidatorsHash

```go
block.ValidatorsHash == MerkleRoot(state.Validators)
```

MerkleRoot of the current validator set that is committing the block.
This can be used to validate the `LastCommit` included in the next block.
Note that before computing the MerkleRoot the validators are sorted
first by voting power (descending), then by address (ascending).

### NextValidatorsHash

```go
block.NextValidatorsHash == MerkleRoot(state.NextValidators)
```

MerkleRoot of the next validator set that will be the validator set that commits the next block.
This is included so that the current validator set gets a chance to sign the
next validator sets Merkle root.
Note that before computing the MerkleRoot the validators are sorted
first by voting power (descending), then by address (ascending).

### ConsensusHash

```go
block.ConsensusHash == state.ConsensusParams.Hash()
```

Hash of the protobuf-encoding of a subset of the consensus parameters.

### AppHash

```go
block.AppHash == state.AppHash
```

Arbitrary byte array returned by the application after executing and commiting the previous block. It serves as the basis for validating any merkle proofs that comes from the ABCI application and represents the state of the actual application rather than the state of the blockchain itself.

The first block's `block.Header.AppHash` is given by `ResponseInitChain.app_hash`.

### LastResultsHash

```go
block.LastResultsHash == MerkleRoot([]ResponseDeliverTx)
```

`LastResultsHash` is the root hash of a Merkle tree built from `ResponseDeliverTx` responses (`Log`,`Info`, `Codespace` and `Events` fields are ignored).

The first block has `block.Header.ResultsHash == MerkleRoot(nil)`, i.e. the hash of an empty input, for RFC-6962 conformance.

## EvidenceHash

```go
block.EvidenceHash == MerkleRoot(block.Evidence)
```

MerkleRoot of the evidence of Byzantine behaviour included in this block.

### ProposerAddress

```go
block.Header.ProposerAddress in state.Validators
```

Address of the original proposer of the block. Must be a current validator.

## Txs

Arbitrary length array of arbitrary length byte-arrays.

## LastCommit

The first height is an exception - it requires the `LastCommit` to be empty:

```go
if block.Header.Height == state.InitialHeight {
  len(b.LastCommit) == 0
}
```

Otherwise, we require:

```go
len(block.LastCommit) == len(state.LastValidators)

talliedVotingPower := 0
for i, commitSig := range block.LastCommit.Signatures {
  if commitSig.Absent() {
    continue
  }

  vote.BlockID == block.LastBlockID

  val := state.LastValidators[i]
  vote.Verify(block.ChainID, val.PubKey) == true

  talliedVotingPower += val.VotingPower
}

talliedVotingPower > (2/3)*TotalVotingPower(state.LastValidators)
```

Includes one vote for every current validator.
All votes must either be for the previous block, nil or absent.
All votes must have a valid signature from the corresponding validator.
The sum total of the voting power of the validators that voted
must be greater than 2/3 of the total voting power of the complete validator set.

The number of votes in a commit is limited to 10000 (see `types.MaxVotesCount`).

### Vote

A vote is a signed message broadcast in the consensus for a particular block at a particular height and round.
When stored in the blockchain or propagated over the network, votes are encoded in Protobuf.
For signing, votes are represented via `CanonicalVote` and also encoded using Protobuf via
`VoteSignBytes` which includes the `ChainID`, and uses a different ordering of
the fields.

We define a method `Verify` that returns `true` if the signature verifies against the pubkey for the `SignBytes`
using the given ChainID:

```go
func (vote *Vote) Verify(chainID string, pubKey crypto.PubKey) error {
 if !bytes.Equal(pubKey.Address(), vote.ValidatorAddress) {
  return ErrVoteInvalidValidatorAddress
 }

 if !pubKey.VerifyBytes(vote.SignBytes(chainID), vote.Signature) {
  return ErrVoteInvalidSignature
 }
 return nil
}
```

where `pubKey.Verify` performs the appropriate digital signature verification of the `pubKey`
against the given signature and message bytes.

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
