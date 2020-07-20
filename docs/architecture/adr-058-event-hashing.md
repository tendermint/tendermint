# ADR 058: Event hashing

## Changelog

- 2020-07-17: initial version

## Context

Before [PR#4845](https://github.com/tendermint/tendermint/pull/4845),
`Header#LastResultsHash` was a root of the Merkle tree built from `DeliverTx`
results. Only `Code`, `Data` fields were included because `Info` and `Log`
fields are non-deterministic.

At some point, we've added events to `ResponseBeginBlock`, `ResponseEndBlock`,
and `ResponseDeliverTx` to give applications a way to attach some additional
information to blocks / transactions.

Many applications seem to have started using them since.

However, before [PR#4845](https://github.com/tendermint/tendermint/pull/4845)
there was no way to prove that certain events were a part of the result. Hence,
[PR#4845](https://github.com/tendermint/tendermint/pull/4845) was opened.

While it's certainly good to be able to prove something, introducing new events
or removing such becomes difficult because it breaks the `LastResultsHash`. It
means that every time you add, remove or update an event, you'll need a
hard-fork. And that is undoubtedly bad for applications, which are evolving and
don't have a stable events set.

## Decision

As a middle ground approach, the proposal is to add the
`Block#LastResultsEvents` consensus parameter that is a list of all events that
are to be hashed in the header.

```
@ proto/tendermint/abci/types.proto:295 @ message BlockParams {
  int64 max_bytes = 1;
  // Note: must be greater or equal to -1
  int64 max_gas = 2;
  // List of events, which will be hashed into the LastResultsHash
  repeated string last_results_events = 3;
}
```

Initially the list is empty. The ABCI application can change it via `InitChain`
or `EndBlock`.

Example:

```go
func (app *MyApp) DeliverTx(req types.RequestDeliverTx) types.ResponseDeliverTx {
    //...
    events := []abci.Event{
        {
            Type: "transfer",
            Attributes: []abci.EventAttribute{
                {Key: []byte("sender"), Value: []byte("Bob"), Index: true},
            },
        },
    }
    return types.ResponseDeliverTx{Code: code.CodeTypeOK, Events: events}
}
```

For "transfer" event to be hashed, the `LastResultsEvents` must contain a
string "transfer".

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
