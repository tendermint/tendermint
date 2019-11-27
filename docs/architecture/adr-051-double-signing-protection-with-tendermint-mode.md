# ADR 51: Double Signing Protection with Tendermint Mode

## Changelog
* 27-11-2019: Initial draft

## Context

There exists two main purposes for this ADR.

1) To introduce the simplest functional abstraction of Tendermint Mode
2) To provide a prevention method for double signing incidents mistakenly executed by validators 


## Decision

We would like to suggest a simple Tendermint mode abstraction with a double signing prevention method.

1) a simple Tendermint mode abstraction
- fullnode mode & validator mode
- which reactor to include for each node
- where to modify in codebase
- cli command and RPC modification

2) a double signing prevention method
- methodology : query recent consensus results to find out whether node's consensus key is used on consensus recently or not
- when to check : everytime when the state machine starts voting reactor
- how to check
- configuration : query heights


## Status

Proposed

## Consequences

### Positive

1) Tendermint mode abstraction
- Node operators can choose mode when they run state machine according to the purpose of the node.
- Mode can prevent mistakes because users have to specify which mode they want to run via flag.
(eg. If a user want to run a validator node, she/he should explicitly write down validator as mode)
- Different mode needs different reactors, resulting in efficient resource usage.

2) double signing prevention method
- Validators can avoid double signing incident by mistakes.
(eg. If another validator node is voting on consensus, starting new validator node with same consensus key will cause 
panic stop of the state machine because consensus votes with the consensus key are found in recent blocks)
- We expect this method will prevent majority of double signing incident by mistakes.


### Negative

1) Tendermint mode abstraction
- Users need to study how each mode operate and which capability it has.

2) double signing prevention method
- When the prevention method is on, restarting a validator node will panic because the node itself voted on consensus
with the same consensus key. So, validators should stop the state machine, wait for some blocks, and then restart the state 
machine to avoid panic stop.

### Neutral

## References

* double-signing protection(https://github.com/tendermint/tendermint/issues/4059)
* Tendermint "mode"(https://github.com/tendermint/tendermint/issues/2237)
