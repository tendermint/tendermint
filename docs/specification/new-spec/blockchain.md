# Tendermint Blockchain

Here we describe the data structures in the Tendermint blockchain and the rules for validating them.

# Data Structures

The Tendermint blockchains consists of a short list of basic data types:
`Block`, `Header`, `Vote`, `BlockID`, and `Signature`.

## Block

A block consists of a header, a list of transactions, a list of votes (the commit),
and a list of evidence if malfeasance (ie. signing conflicting votes).

```
type Block struct {
    Header      Header
    Txs         [][]byte
    LastCommit  []Vote
    Evidence    []Evidence
}
```

## Header

A block header contains metadata about the block and about the consensus, as well as commitments to
the data in the current block, the previous block, and the results returned by the application:

```
type Header struct {
    // block metadata
    Version             string  // Version string
    ChainID             string  // ID of the chain
    Height              int64   // current block height
    Time                int64   // UNIX time, in millisconds

    // current block
    NumTxs              int64   // Number of txs in this block
    TxHash              []byte  // SimpleMerkle of the block.Txs
    LastCommitHash      []byte  // SimpleMerkle of the block.LastCommit

    // previous block
    TotalTxs            int64   // prevBlock.TotalTxs + block.NumTxs
    LastBlockID         BlockID // BlockID of prevBlock

    // application
    ResultsHash         []byte  // SimpleMerkle of []abci.Result from prevBlock
    AppHash             []byte  // Arbitrary state digest
    ValidatorsHash      []byte  // SimpleMerkle of the ValidatorSet
    ConsensusParamsHash []byte  // SimpleMerkle of the ConsensusParams

    // consensus
    Proposer            []byte  // Address of the block proposer
    EvidenceHash        []byte  // SimpleMerkle of []Evidence
}
```

Further details on each of this fields is taken up below.

## BlockID

The `BlockID` contains two distinct Merkle roots of the block.
The first, used as the block's main hash, is the Merkle root
of all the fields in the header. The second, used for secure gossipping of
the block during consensus, is the Merkle root of the complete serialized block
cut into parts. The `BlockID` includes these two hashes, as well as the number of
parts.

```
type BlockID struct {
    HeaderHash  []byte
    NumParts    int32
    PartsHash   []byte
}
```

## Vote

A vote is a signed message from a validator for a particular block.
The vote includes information about the validator signing it.

```
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
a prevote has `vote.Type == 1` and
a precommit has `vote.Type == 2`.

## Signature

Tendermint allows for multiple signature schemes to be used by prepending a single type-byte
to the signature bytes. Different signatures may also come with fixed or variable lengths.
Currently, Tendermint supports Ed25519 and Secp256k1.

### ED25519

An ED25519 signature has `Type == 0x1`. It looks like:

```
// Implements Signature
type Ed25519Signature struct {
    Type        int8        = 0x1
    Signature   [64]byte
}
```

where `Signature` is the 64 byte signature.

### Secp256k1

A `Secp256k1` signature has `Type == 0x2`. It looks like:

```
// Implements Signature
type Secp256k1Signature struct {
    Type        int8        = 0x2
    Signature   []byte
}
```

where `Signature` is the DER encoded signature, ie:

```
0x30 <length of whole message> <0x02> <length of R> <R> 0x2 <length of S> <S>.
```

# Validation

Here we describe the validation rules for every element in block.
Blocks which do not satisfy these rules are considered invalid.

We abuse notation by using something that looks like Go, supplemented with English.
A statement such as `x == y` is an assertion - if it fails, the item is invalid.

We refer to certain globally available objects:
`block` is the block under consideration,
`prevBlock` is the `block` at the previous height,
`state` keeps track of the validator set
and the consensus parameters as they are updated by the application,
and `app` is the set of responses returned by the application during the
execution of the `prevBlock`. Elements of an object are accessed as expected,
ie. `block.Header`. See the definitions of `state` and `app` elsewhere.

## Header

A Header is valid if its corresponding fields are valid.

### Version

Arbitrary string.

### ChainID

Arbitrary constant string.

### Height

```
block.Header.Height > 0
block.Header.Height == prevBlock.Header.Height + 1
```

### Time

The median of the timestamps of the valid votes in the block.LastCommit.
Corresponds to the number of milliseconds since January 1, 1970.
Increments with every new block.

### NumTxs

```
block.Header.NumTxs == len(block.Txs)
```

Number of transactions included in the block.

### TxHash

```
block.Header.TxHash == SimpleMerkleRoot(block.Txs)
```

Simple Merkle root of the transactions in the block.

### LastCommitHash

```
block.Header.LastCommitHash == SimpleMerkleRoot(block.LastCommit)
```

Simple Merkle root of the votes included in the block.
These are the votes that committed the previous block.

### TotalTxs

```
block.Header.TotalTxs == prevBlock.Header.TotalTxs + block.header.NumTxs
```

The cumulative sum of all transactions included in this blockchain.

### LastBlockID

```
parts := MakeBlockParts(block, state.ConsensusParams.BlockGossip.BlockPartSize)
block.HeaderLastBlockID == BlockID{
    SimpleMerkleRoot(block.Header),
    len(parts),
    SimpleMerkleRoot(parts),
}
```

Previous block's BlockID. Note it depends on the ConsensusParams,
which are held in the `state` and may be updated by the application.

### ResultsHash

```
block.ResultsHash == SimpleMerkleRoot(app.Results)
```

Simple Merkle root of the results of the transactions in the previous block.

### AppHash

```
block.AppHash == app.AppHash
```

Arbitrary byte array returned by the application after executing and commiting the previous block.

### ValidatorsHash

```
block.ValidatorsHash == SimpleMerkleRoot(state.Validators)
```

Simple Merkle root of the current validator set that is committing the block.
This can be used to validate the `LastCommit` included in the next block.
May be updated by the applicatoin.

### ConsensusParamsHash

```
block.ValidatorsHash == SimpleMerkleRoot(state.ConsensusParams)
```

Simple Merkle root of the consensus parameters.
May be updated by the application.

### Proposer

Original proposer of the block.

TODO

### EvidenceHash

```
block.EvidenceHash == SimpleMerkleRoot(block.Evidence)
```

Simple Merkle root of the evidence of Byzantine behaviour included in this block.

## Txs

Arbitrary length array of arbitrary length byte-arrays.

## LastCommit

The first height is an exception - it requires the LastCommit to be empty:

```
if block.Header.Height == 1 {
    len(b.LastCommit) == 0
}
```

Otherwise, we require:

```
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

    pubkey, votingPower := state.LastValidators.Get(i)
    VerifyVoteSignature(block.ChainID, vote, pubkey)

    talliedVotingPower += votingPower
}

talliedVotingPower > (2/3) * state.LastValidators.TotalVotingPower()
```

Includes one (possibly nil) vote for every current validator.
Non-nil votes must be Precommits.
All votes must be for the same height and round.
All votes must be for the previous block.
All votes must have a valid signature from the corresponding validator.
The sum total of the voting power of the validators that voted
must be greater than 2/3 of the total voting power of the complete validator set.

## Evidence


```


```

Every piece of evidence contains two conflicting votes from a single validator that
was active at the height indicated in the votes.
The votes must not be too old.

