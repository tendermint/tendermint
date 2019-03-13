# ADR 037: Deliver Block

Author: Daniil Lashin (@danil-lashin)

## Changelog

13-03-2019: Initial draft

## Context

Initial conversation: https://github.com/tendermint/tendermint/issues/2901

Some applications can handle transactions in parallel, or at least some 
part of tx processing can be parallelized. Now it is not possible for developer
to execute txs in parallel because Tendermint delivers them consequentially.

## Decision

Now Tendermint have `BeginBlock`, `EndBlock`, `Commit`, `DeliverTx` steps 
while executing block. This doc proposes merging this steps into one `DeliverBlock` 
step. It will allow developers of applications to decide how they want to 
execute transactions (in parallel or consequentially) Also it will simplify and 
speed up communications between application and Tendermint. 

## Status

Draft

## Consequences

### Positive

- txs can be handled in parallel
- simpler interface
- faster communications between Tendermint and application

### Negative

- breaking change, all applications should be adapted to new interface

### Neutral

