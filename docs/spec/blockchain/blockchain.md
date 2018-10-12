# Blockchain

Here we describe the data structures in the Tendermint blockchain and the rules for validating them.

## Data Structures

The Tendermint blockchains consists of a short list of basic data types:

- `Block`
- `Header`
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

Note the `LastCommit` is the set of  votes that committed the last block.

## Header

A block header contains metadata about the block and about the consensus, as well as commitments to
the data in the current block, the previous block, and the results returned by the application:

```go
type Header struct {
	// basic block info
	ChainID  string
	Height   int64
	Time     Time
	NumTxs   int64
	TotalTxs int64

	// prev block info
	LastBlockID BlockID

	// hashes of block data
	LastCommitHash []byte // commit from validators from the last block
	DataHash       []byte // Merkle root of transactions

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

## BlockID

The `BlockID` contains two distinct Merkle roots of the block.
The first, used as the block's main hash, is the Merkle root
of all the fields in the header. The second, used for secure gossipping of
the block during consensus, is the Merkle root of the complete serialized block
cut into parts. The `BlockID` includes these two hashes, as well as the number of
parts.

```go
type BlockID struct {
    Hash []byte
    Parts PartsHeader
}

type PartsHeader struct {
    Hash []byte
    Total int32
}
```

TODO: link to details of merkle sums.

## Time

Tendermint uses the
[Google.Protobuf.WellKnownTypes.Timestamp](https://developers.google.com/protocol-buffers/docs/reference/csharp/class/google/protobuf/well-known-types/timestamp)
format, which uses two integers, one for Seconds and for Nanoseconds.

NOTE: there is currently a small divergence between Tendermint and the
Google.Protobuf.WellKnownTypes.Timestamp that should be resolved. See [this
issue](https://github.com/tendermint/go-amino/issues/223) for details.

## Data

Data is just a wrapper for a list of transactions, where transactions are
arbitrary byte arrays:

```
type Data struct {
    Txs [][]byte
}
```

## Commit

Commit is a simple wrapper for a list of votes, with one vote for each
validator. It also contains the relevant BlockID:

```
type Commit struct {
    BlockID     BlockID
    Precommits  []Vote
}
```

NOTE: this will likely change to reduce the commit size by eliminating redundant
information - see [issue #1648](https://github.com/tendermint/tendermint/issues/1648).

## Vote

A vote is a signed message from a validator for a particular block.
The vote includes information about the validator signing it.

```go
type Vote struct {
    ValidatorAddress    []byte
    ValidatorIndex      int
    Height              int64
    Round               int
    Timestamp           Time
    Type                int8
    BlockID             BlockID
    Signature           []byte
}
```

There are two types of votes:
a _prevote_ has `vote.Type == 1` and
a _precommit_ has `vote.Type == 2`.

## Signature

Signatures in Tendermint are raw bytes representing the underlying signature.
The only signature scheme currently supported for Tendermint validators is
ED25519. The signature is the raw 64-byte ED25519 signature.

## EvidenceData

EvidenceData is a simple wrapper for a list of evidence:

```
type EvidenceData struct {
    Evidence []Evidence
}
```

## Evidence

Evidence in Tendermint is implemented as an interface.
This means any evidence is encoded using its Amino prefix.
There is currently only a single type, the `DuplicateVoteEvidence`.

```
// amino name: "tendermint/DuplicateVoteEvidence"
type DuplicateVoteEvidence struct {
	PubKey PubKey
	VoteA  Vote
	VoteB  Vote
}
```

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
See [here](https://github.com/tendermint/tendermint/blob/master/docs/spec/blockchain/state.md) for the definition of `state`.

### Header

A Header is valid if its corresponding fields are valid.

### ChainID

```
len(block.ChainID) < 50
```

ChainID must be maximum 50 UTF-8 symbols.

### Height

```go
block.Header.Height > 0
block.Header.Height == prevBlock.Header.Height + 1
```

The height is an incrementing integer. The first block has `block.Header.Height == 1`.

### Time

```
block.Header.Timestamp >= prevBlock.Header.Timestamp + 1 ms
block.Header.Timestamp == MedianTime(block.LastCommit, state.LastValidators)
```

The block timestamp must be monotonic.
It must equal the weighted median of the timestamps of the valid votes in the block.LastCommit.

Note: the timestamp of a vote must be greater by at least one millisecond than that of the
block being voted on.

The timestamp of the first block must be equal to the genesis time (since
there's no votes to compute the median).

```
if block.Header.Height == 1 {
    block.Header.Timestamp == genesisTime
}
```

See the section on [BFT time](../consensus/bft-time.md) for more details.

### NumTxs

```go
block.Header.NumTxs == len(block.Txs.Txs)
```

Number of transactions included in the block.

### TotalTxs

```go
block.Header.TotalTxs == prevBlock.Header.TotalTxs + block.Header.NumTxs
```

The cumulative sum of all transactions included in this blockchain.

The first block has `block.Header.TotalTxs = block.Header.NumberTxs`.

### LastBlockID

LastBlockID is the previous block's BlockID:

```go
prevBlockParts := MakeParts(prevBlock, state.LastConsensusParams.BlockGossip.BlockPartSize)
block.Header.LastBlockID == BlockID {
    Hash: SimpleMerkleRoot(prevBlock.Header),
    PartsHeader{
        Hash: SimpleMerkleRoot(prevBlockParts),
        Total: len(prevBlockParts),
    },
}
```

Note: it depends on the ConsensusParams,
which are held in the `state` and may be updated by the application.

The first block has `block.Header.LastBlockID == BlockID{}`.

### LastCommitHash

```go
block.Header.LastCommitHash == SimpleMerkleRoot(block.LastCommit)
```

Simple Merkle root of the votes included in the block.
These are the votes that committed the previous block.

The first block has `block.Header.LastCommitHash == []byte{}`

### DataHash

```go
block.Header.DataHash == SimpleMerkleRoot(block.Txs.Txs)
```

Simple Merkle root of the transactions included in the block.

### ValidatorsHash

```go
block.ValidatorsHash == SimpleMerkleRoot(state.Validators)
```

Simple Merkle root of the current validator set that is committing the block.
This can be used to validate the `LastCommit` included in the next block.

### NextValidatorsHash

```go
block.NextValidatorsHash == SimpleMerkleRoot(state.NextValidators)
```

Simple Merkle root of the next validator set that will be the validator set that commits the next block.
This is included so that the current validator set gets a chance to sign the
next validator sets Merkle root.

### ConsensusParamsHash

```go
block.ConsensusParamsHash == SimpleMerkleRoot(state.ConsensusParams)
```

Simple Merkle root of the consensus parameters.

### AppHash

```go
block.AppHash == state.AppHash
```

Arbitrary byte array returned by the application after executing and commiting the previous block. It serves as the basis for validating any merkle proofs that comes from the ABCI application and represents the state of the actual application rather than the state of the blockchain itself.

The first block has `block.Header.AppHash == []byte{}`.

### LastResultsHash

```go
block.ResultsHash == SimpleMerkleRoot(state.LastResults)
```

Simple Merkle root of the results of the transactions in the previous block.

The first block has `block.Header.ResultsHash == []byte{}`.

## EvidenceHash

```go
block.EvidenceHash == SimpleMerkleRoot(block.Evidence)
```

Simple Merkle root of the evidence of Byzantine behaviour included in this block.

### ProposerAddress

```go
block.Header.ProposerAddress in state.Validators
```

Address of the original proposer of the block. Must be a current validator.

## Txs

Arbitrary length array of arbitrary length byte-arrays.

## LastCommit

The first height is an exception - it requires the LastCommit to be empty:

```go
if block.Header.Height == 1 {
    len(b.LastCommit) == 0
}
```

Otherwise, we require:

```go
len(block.LastCommit) == len(state.LastValidators)
talliedVotingPower := 0
for i, vote := range block.LastCommit{
    if vote == nil{
        continue
    }
    vote.Type == 2
    vote.Height == block.LastCommit.Height()
    vote.Round == block.LastCommit.Round()
    vote.BlockID == block.LastBlockID

    val := state.LastValidators[i]
    vote.Verify(block.ChainID, val.PubKey) == true

    talliedVotingPower += val.VotingPower
}

talliedVotingPower > (2/3) * TotalVotingPower(state.LastValidators)
```

Includes one (possibly nil) vote for every current validator.
Non-nil votes must be Precommits.
All votes must be for the same height and round.
All votes must be for the previous block.
All votes must have a valid signature from the corresponding validator.
The sum total of the voting power of the validators that voted
must be greater than 2/3 of the total voting power of the complete validator set.

### Vote

A vote is a signed message broadcast in the consensus for a particular block at a particular height and round.
When stored in the blockchain or propagated over the network, votes are encoded in Amino.
For signing, votes are represented via `CanonicalVote` and also encoded using amino (protobuf compatible) via 
`Vote.SignBytes` which includes the `ChainID`.

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

## Evidence

There is currently only one kind of evidence, `DuplicateVoteEvidence`.

DuplicateVoteEvidence `ev` is valid if

- `ev.VoteA` and `ev.VoteB` can be verified with `ev.PubKey`
- `ev.VoteA` and `ev.VoteB` have the same `Height, Round, Address, Index, Type`
- `ev.VoteA.BlockID != ev.VoteB.BlockID`
- `(block.Height - ev.VoteA.Height) < MAX_EVIDENCE_AGE`

# Execution

Once a block is validated, it can be executed against the state.

The state follows this recursive equation:

```go
state(1) = InitialState
state(h+1) <- Execute(state(h), ABCIApp, block(h))
```

where `InitialState` includes the initial consensus parameters and validator set,
and `ABCIApp` is an ABCI application that can return results and changes to the validator
set (TODO). Execute is defined as:

```go
Execute(s State, app ABCIApp, block Block) State {
    // Fuction ApplyBlock executes block of transactions against the app and returns the new root hash of the app state,
    // modifications to the validator set and the changes of the consensus parameters.
    AppHash, ValidatorChanges, ConsensusParamChanges := app.ApplyBlock(block)

    return State{
        LastResults: abciResponses.DeliverTxResults,
        AppHash: AppHash,
        LastValidators: state.Validators,
        Validators: state.NextValidators,
        NextValidators: UpdateValidators(state.NextValidators, ValidatorChanges),
        ConsensusParams: UpdateConsensusParams(state.ConsensusParams, ConsensusParamChanges),
    }
}
```
