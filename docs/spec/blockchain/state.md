# State

## State

The state contains information whose cryptographic digest is included in block headers, and thus is
necessary for validating new blocks. For instance, the validators set and the results of
transactions are never included in blocks, but their Merkle roots are - the state keeps track of them.

Note that the `State` object itself is an implementation detail, since it is never
included in a block or gossipped over the network, and we never compute
its hash. Thus we do not include here details of how the `State` object is
persisted or queried. That said, the types it contains are part of the specification, since
their Merkle roots are included in blocks and their values are used in
validation.

```go
type State struct {
    Version     Version
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
}
```

`Result` is the result of executing a transaction against the application.
It returns a result code and an arbitrary byte array (ie. a return value).

NOTE: the Result needs to be updated to include more fields returned from
processing transactions, like gas variables and tags - see
[issue 1007](https://github.com/tendermint/tendermint/issues/1007).

### Validator

A validator is an active participant in the consensus with a public key and a voting power.
Validator's also contain an address field, which is a hash digest of the PubKey.

```go
type Validator struct {
    Address     []byte
    PubKey      PubKey
    VotingPower int64
}
```

When hashing the Validator struct, the address is not included,
because it is redundant with the pubkey.

The `state.Validators`, `state.LastValidators`, and `state.NextValidators`, must always by sorted by validator address,
so that there is a canonical order for computing the MerkleRoot.

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
When hashed, only a subset of the params are included, to allow the params to
evolve without breaking the header.

```go
type ConsensusParams struct {
	BlockSize
	Evidence
	Validator
}

type hashedParams struct {
    BlockMaxBytes int64
    BlockMaxGas   int64
}

func (params ConsensusParams) Hash() []byte {
    SHA256(hashedParams{
        BlockMaxBytes: params.BlockSize.MaxBytes,
        BlockMaxGas: params.BlockSize.MaxGas,
    })
}

type BlockSize struct {
	MaxBytes        int64
	MaxGas          int64
}

type Evidence struct {
	MaxAge int64
}

type Validator struct {
	PubKeyTypes []string
}

type ValidatorParams struct {
	PubKeyTypes []string
}
```

#### BlockSize

The total size of a block is limited in bytes by the `ConsensusParams.BlockSize.MaxBytes`.
Proposed blocks must be less than this size, and will be considered invalid
otherwise.

Blocks should additionally be limited by the amount of "gas" consumed by the
transactions in the block, though this is not yet implemented.

#### Evidence

For evidence in a block to be valid, it must satisfy:

```
block.Header.Height - evidence.Height < ConsensusParams.Evidence.MaxAge
```

#### Validator

Validators from genesis file and `ResponseEndBlock` must have pubkeys of type ∈
`ConsensusParams.Validator.PubKeyTypes`.
