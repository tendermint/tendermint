# Tendermint Blockchain

Here we describe the data structures in the Tendermint blockchain and the rules for validating them.

## Data Structures

The Tendermint blockchains consists of a short list of basic data types:

- `Block`
- `Header`
- `Vote`
- `BlockID`
- `Signature`
- `Evidence`

## Block

A block consists of a header, a list of transactions, a list of votes (the commit),
and a list of evidence of malfeasance (ie. signing conflicting votes).

```go
type Block struct {
    Header      Header
    Txs         [][]byte
    LastCommit  []Vote
    Evidence    []Evidence
}
```

The signatures returned along with block `X` are those validating block
`X-1`. This can be a little confusing, but consider that
the `Header` also contains the `LastCommitHash`. It would be impossible
for a Header to include the commits that sign it, as it would cause an
infinite loop here. But when we get block `X`, we find
`Header.LastCommitHash`, which must match the hash of `LastCommit`.

## Header

A block header contains metadata about the block and about the consensus, as well as commitments to
the data in the current block, the previous block, and the results returned by the application:

```go
type Header struct {
	// basic block info
	ChainID  string    `json:"chain_id"`
	Height   int64     `json:"height"`
	Time     time.Time `json:"time"`
	NumTxs   int64     `json:"num_txs"`
	TotalTxs int64     `json:"total_txs"`

	// prev block info
	LastBlockID BlockID `json:"last_block_id"`

	// hashes of block data
	LastCommitHash cmn.HexBytes `json:"last_commit_hash"` // commit from validators from the last block
	DataHash       cmn.HexBytes `json:"data_hash"`        // transactions

	// hashes from the app output from the prev block
	ValidatorsHash     cmn.HexBytes `json:"validators_hash"`      // validators for the current block
	NextValidatorsHash cmn.HexBytes `json:"next_validators_hash"` // validators for the next block
	ConsensusHash      cmn.HexBytes `json:"consensus_hash"`       // consensus params for current block
	AppHash            cmn.HexBytes `json:"app_hash"`             // state after txs from the previous block
	LastResultsHash    cmn.HexBytes `json:"last_results_hash"`    // root hash of all results from the txs from the previous block

	// consensus info
	EvidenceHash    cmn.HexBytes `json:"evidence_hash"`    // evidence included in the block
	ProposerAddress Address      `json:"proposer_address"` // original proposer of the block
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

## Vote

A vote is a signed message from a validator for a particular block.
The vote includes information about the validator signing it.

```go
type Vote struct {
    Timestamp   int64
    Address     []byte
    Index       int
    Height      int64
    Round       int
    Type        int8
    BlockID     BlockID
    Signature   Signature
}
```

There are two types of votes:
a _prevote_ has `vote.Type == 1` and
a _precommit_ has `vote.Type == 2`.

## Signature

Tendermint allows for multiple signature schemes to be used by prepending a single type-byte
to the signature bytes. Different signatures may also come with fixed or variable lengths.
Currently, Tendermint supports Ed25519 and Secp256k1.

### ED25519

An ED25519 signature has `Type == 0x1`. It looks like:

```go
// Implements Signature
type Ed25519Signature struct {
    Type        int8        = 0x1
    Signature   [64]byte
}
```

where `Signature` is the 64 byte signature.

### Secp256k1

A `Secp256k1` signature has `Type == 0x2`. It looks like:

```go
// Implements Signature
type Secp256k1Signature struct {
    Type        int8        = 0x2
    Signature   []byte
}
```

where `Signature` is the DER encoded signature, ie:

```hex
0x30 <length of whole message> <0x02> <length of R> <R> 0x2 <length of S> <S>.
```

## Evidence

TODO

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

### Version

Arbitrary string.

### ChainID

Arbitrary constant string.

### Height

```go
block.Header.Height > 0
block.Header.Height == prevBlock.Header.Height + 1
```

The height is an incrementing integer. The first block has `block.Header.Height == 1`.

### Time

The median of the timestamps of the valid votes in the block.LastCommit.
Corresponds to the number of nanoseconds, with millisecond resolution, since January 1, 1970.

Note: the timestamp of a vote must be greater by at least one millisecond than that of the
block being voted on.

### NumTxs

```go
block.Header.NumTxs == len(block.Txs)
```

Number of transactions included in the block.

### TxHash

```go
block.Header.TxHash == SimpleMerkleRoot(block.Txs)
```

Simple Merkle root of the transactions in the block.

### LastCommitHash

```go
block.Header.LastCommitHash == SimpleMerkleRoot(block.LastCommit)
```

Simple Merkle root of the votes included in the block.
These are the votes that committed the previous block.

The first block has `block.Header.LastCommitHash == []byte{}`

### DataHash

The `DataHash` can provide a nice check on the
[Data](https://godoc.org/github.com/tendermint/tendermint/types#Data)
returned in this same block. If you are subscribed to new blocks, via
tendermint RPC, in order to display or process the new transactions you
should at least validate that the `DataHash` is valid. If it is
important to verify autheniticity, you must wait for the `LastCommit`
from the next block to make sure the block header (including `DataHash`)
was properly signed.

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

### ResultsHash

```go
block.ResultsHash == SimpleMerkleRoot(state.LastResults)
```

Simple Merkle root of the results of the transactions in the previous block.

The first block has `block.Header.ResultsHash == []byte{}`.

### AppHash

```go
block.AppHash == state.AppHash
```

Arbitrary byte array returned by the application after executing and commiting the previous block. It serves as the basis for validating any merkle proofs that comes from the ABCI application and represents the state of the actual application rather than the state of the blockchain itself.

The first block has `block.Header.AppHash == []byte{}`.

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
Modifications to the validator set are defined by the application.

### ConsensusParamsHash

```go
block.ConsensusParamsHash == SimpleMerkleRoot(state.ConsensusParams)
```

Simple Merkle root of the consensus parameters.
May be updated by the application.

### ProposerAddress

```go
block.Header.ProposerAddress in state.Validators
```

Address of the original proposer of the block. Must be a current validator.

NOTE: we also need to track the round.

## EvidenceHash

```go
block.EvidenceHash == SimpleMerkleRoot(block.Evidence)
```

Simple Merkle root of the evidence of Byzantine behaviour included in this block.

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
When stored in the blockchain or propagated over the network, votes are encoded in TMBIN.
For signing, votes are encoded in JSON, and the ChainID is included, in the form of the `CanonicalSignBytes`.

We define a method `Verify` that returns `true` if the signature verifies against the pubkey for the CanonicalSignBytes
using the given ChainID:

```go
func (v Vote) Verify(chainID string, pubKey PubKey) bool {
    return pubKey.Verify(v.Signature, CanonicalSignBytes(chainID, v))
}
```

where `pubKey.Verify` performs the appropriate digital signature verification of the `pubKey`
against the given signature and message bytes.

## Evidence

There is currently only one kind of evidence:

```
// amino: "tendermint/DuplicateVoteEvidence"
type DuplicateVoteEvidence struct {
	PubKey crypto.PubKey
	VoteA  *Vote
	VoteB  *Vote
}
```

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
