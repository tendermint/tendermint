# Tendermint State

## State

The state contains information whose cryptographic digest is included in block headers, and thus is
necessary for validating new blocks. For instance, the validators set and the results of
transactions are never included in blocks, but their Merkle roots are - the state keeps track of them.

Note that the `State` object itself is an implementation detail, since it is never
included in a block or gossipped over the network, and we never compute
its hash. However, the types it contains are part of the specification, since
their Merkle roots are included in blocks.

Details on an implementation of `State` with persistence is forthcoming, see [this issue](https://github.com/tendermint/tendermint/issues/1152)

```go
type State struct {
    LastResults []Result
    AppHash []byte

    LastValidators []Validator
    Validators []Validator
    NextValidators []Validator

    ConsensusParams ConsensusParams
}
```

### Result

```go
type Result struct {
    Code uint32
    Data []byte
    Tags []KVPair
}

type KVPair struct {
    Key     []byte
    Value   []byte
}
```

`Result` is the result of executing a transaction against the application.
It returns a result code, an arbitrary byte array (ie. a return value),
and a list of key-value pairs ordered by key. The key-value pairs, or tags,
can be used to index transactions according to their "effects", which are
represented in the tags.

### Validator

A validator is an active participant in the consensus with a public key and a voting power.
Validator's also contain an address which is derived from the PubKey:

```go
type Validator struct {
    Address     []byte
    PubKey      PubKey
    VotingPower int64
}
```

The `state.Validators`, `state.LastValidators`, and `state.NextValidators`, must always by sorted by validator address,
so that there is a canonical order for computing the SimpleMerkleRoot.

We also define a `TotalVotingPower` function, to return the total voting power:

```go
func TotalVotingPower(vals []Validators) int64{
    sum := 0
    for v := range vals{
        sum += v.VotingPower
    }
    return sum
}
```

### ConsensusParams

ConsensusParams define various limits for blockchain data structures.
Like validator sets, they are set during genesis and can be updated by the application through ABCI.

```
type ConsensusParams struct {
	BlockSize
	TxSize
	BlockGossip
	EvidenceParams
}

type BlockSize struct {
	MaxBytes        int
	MaxGas          int64
}

type TxSize struct {
	MaxBytes int
	MaxGas   int64
}

type BlockGossip struct {
	BlockPartSizeBytes int
}

type EvidenceParams struct {
	MaxAge int64
}
```

#### BlockSize

The total size of a block is limitted in bytes by the `ConsensusParams.BlockSize.MaxBytes`.
Proposed blocks must be less than this size, and will be considered invalid
otherwise.

Blocks should additionally be limitted by the amount of "gas" consumed by the
transactions in the block, though this is not yet implemented.

#### TxSize

These parameters are not yet enforced and may disappear. See [issue
#2347](https://github.com/tendermint/tendermint/issues/2347).

#### BlockGossip

When gossipping blocks in the consensus, they are first split into parts. The
size of each part is `ConsensusParams.BlockGossip.BlockPartSizeBytes`.

#### EvidenceParams

For evidence in a block to be valid, it must satisfy:

```
block.Header.Height - evidence.Height < ConsensusParams.EvidenceParams.MaxAge
```
