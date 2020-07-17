# ADR 058: Event hashing

## Changelog

- 2020-07-17: initial version

## Context

Before [PR#4845](https://github.com/tendermint/tendermint/pull/4845),
`Header#LastResultsHash` was a root of the Merkle tree built from `DeliverTx`
results. Only `Code`, `Data` fields were included because `Info` and `Log`
fields are non-deterministic. `GasWanted` and `GasUsed` fields appeared later.

At some point, we've added events to `ResponseBeginBlock`, `ResponseEndBlock`,
and `ResponseDeliverTx` to give applications a way to attach some additional
information.

Many applications seem to have started using them since.

However, before [PR#4845](https://github.com/tendermint/tendermint/pull/4845)
there was no way to prove that certain events were a part of the result.

Hence, [PR#4845](https://github.com/tendermint/tendermint/pull/4845) was
opened.

While it's certainly good to be able to prove something, introducing new events
or removing such becomes difficult because it breaks the `LastResultsHash`. And
that is certainly bad for applications, which haven't matured enough.

## Decision

As a middle ground approach, we can add the `Block#LastResultsEvents` consensus
parameter that is a list of all events that are to be hashed in the header.

## Status

Proposed

## Consequences

### Positive

1. networks can perform parameter change proposals to update this list as new events are added
2. allows networks to avoid having to do hard-forks
3. events can still be added at-will to the application w/o breaking anything

### Negative

1. yet another consensus parameter.

### Neutral

## References
