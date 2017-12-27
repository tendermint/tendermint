# Tendermint State

## State

The state contains information whose cryptographic digest is included in block headers,
and thus is necessary for validating new blocks.
For instance, the Merkle root of the results from executing the previous block, or the Merkle root of the current validators.
While neither the results of transactions now the validators are ever included in the blockchain itself,
the Merkle roots are, and hence we need a separate data structure to track them.

```
type State struct {
    LastResults []Result
    AppHash []byte

    Validators []Validator
    LastValidators []Validator

    ConsensusParams ConsensusParams
}
```

### Result

```
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

```
type Validator struct {
    Address     []byte
    PubKey      PubKey
    VotingPower int64
}
```

The `state.Validators` and `state.LastValidators` must always by sorted by validator address,
so that there is a canonical order for computing the SimpleMerkleRoot.

We also define a `TotalVotingPower` function, to return the total voting power:

```
func TotalVotingPower(vals []Validators) int64{
    sum := 0
    for v := range vals{
        sum += v.VotingPower
    }
    return sum
}
```

### PubKey

TODO:

### ConsensusParams

TODO:

## Execution

We define an `Execute` function that takes a state and a block,
executes the block against the application, and returns an updated state.

```
Execute(s State, app ABCIApp, block Block) State {
    abciResponses := app.ApplyBlock(block)

    return State{
        LastResults: abciResponses.DeliverTxResults,
        AppHash: abciResponses.AppHash,
        Validators: UpdateValidators(state.Validators, abciResponses.ValidatorChanges),
        LastValidators: state.Validators,
        ConsensusParams: UpdateConsensusParams(state.ConsensusParams, abci.Responses.ConsensusParamChanges),
    }
}

type ABCIResponses struct {
    DeliverTxResults        []Result
    ValidatorChanges        []Validator
    ConsensusParamChanges   ConsensusParams
    AppHash                 []byte
}
```
