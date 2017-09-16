# ADR 005: Consensus Params

## Context

Consensus critical parameters controlling blockchain capacity have until now been hard coded, loaded from a local config, or neglected.
Since they may be need to be different in different networks, and potentially to evolve over time within
networks, we seek to initialize them in a genesis file, and expose them through the ABCI.

While we have some specific parameters now, like maximum block and transaction size, we expect to have more in the future,
such as a period over which evidence is valid, or the frequency of checkpoints. 

## Decision

### ConsensusParams

A new `ConsensusParams` is optionally included in the `genesis.json` file,
and loaded into the `State`. Any items not included are set to their default value.
A value of 0 is undefined (see ABCI, below). A value of -1 is used to indicate the parameter does not apply.
No consensus critical parameters should ever be found in the `config.toml`.

```
type ConsensusParams struct {
    BlockSizeParams
    TxSizeParams
    BlockGossipParams
}

type BlockSizeParams struct {
    BlockSizeBytes int
    BlockSizeTxs int
    BlockSizeGas int
}

type TxSizeParams struct {
    TxSizeBytes int
    TxSizeGas int
}

type BlockGossipParams struct {
    BlockPartSizeBytes int
}
```

The `ConsensusParams` can evolve over time by adding new structs that cover different aspects of the consensus rules.

### ABCI

#### InitChain

InitChain currently takes the initial validator set. It should be extended to also take the ConsensusParams.
In fact, it might as well just consume the whole Genesis.

#### EndBlock

The EndBlock response includes a `ConsensusParams`, which includes BlockSizeParams and TxSizeParams, but not BlockGossipParams.
Other param struct can be added to `ConsensusParams` in the future.
The `0` value is used to denote no change. 
Any other value will update that parameter in the `State.ConsensusParams`, to be applied for the next block.
Tendermint should have hard-coded upper limits as sanity checks.

## Status

Proposed.

## Consequences

### Positive

- Alternative capacity limits and consensus parameters can be specified without re-compiling the software.
- They can also change over time under the control of the application

### Negative

- More exposed parameters is more complexity
- Different rules at different heights in the blockchain complicates fast sync

### Neutral
