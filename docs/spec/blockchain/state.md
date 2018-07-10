# Tendermint State

## State

The state contains information whose cryptographic digest is included in block headers, and thus is
necessary for validating new blocks. For instance, the set of validators and the results of
transactions are never included in blocks, but their Merkle roots are - the state keeps track of them.

Note that the `State` object itself is an implementation detail, since it is never
included in a block or gossipped over the network, and we never compute
its hash. However, the types it contains are part of the specification, since
their Merkle roots are included in blocks.

For details on an implementation of `State` with persistence, see TODO

```go
type State struct {
    LastResults []Result
    AppHash []byte

    Validators []Validator
    LastValidators []Validator

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

The `state.Validators` and `state.LastValidators` must always by sorted by validator address,
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

TODO
