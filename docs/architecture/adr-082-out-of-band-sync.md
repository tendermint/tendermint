# ADR 082: Supporting out of band state sync

## Changelog

- 2022-10-31: Initial Draft (@cmwaters)

## Status

> An architecture decision is considered "proposed" when a PR containing the ADR
> is submitted. When merged, an ADR must have a status associated with it, which
> must be one of: "Accepted", "Rejected", "Deprecated" or "Superseded".
>
> An accepted ADR's implementation status must be tracked via a tracking issue,
> milestone or project board (only one of these is necessary). For example:
>
>     Accepted
>
>     [Tracking issue](https://github.com/tendermint/tendermint/issues/123)
>     [Milestone](https://github.com/tendermint/tendermint/milestones/123)
>     [Project board](https://github.com/orgs/tendermint/projects/123)
>
> Rejected ADRs are captured as a record of recommendations that we specifically
> do not (and possibly never) want to implement. The ADR itself must, for
> posterity, include reasoning as to why it was rejected.
>
> If an ADR is deprecated, simply write "Deprecated" in this section. If an ADR
> is superseded by one or more other ADRs, provide local a reference to those
> ADRs, e.g.:
>
>     Superseded by [ADR 123](./adr-123.md)

Accepted

## Context

> This section contains all the context one needs to understand the current state,
> and why there is a problem. It should be as succinct as possible and introduce
> the high level idea behind the solution.

Whenever Tendermint begins, it initiates a handshake with the application via `Info` to gauge version compatibility and recieve the last height and app hash of the application. Tendermint then begins a replay protocol which aims to sync the heights of Tendermint's state and block store and the application's current height. 

When initially designed, one invariant was that the applications current height should never exceed
Tendermint's height. The protocol would error and the node would shut down if this were to happen. It seemed
appropriate at the time that Tendermint which would provide blocks either through block sync or consensus
would always be ahead. However, in light of state sync and other protocols for bootstrapping an application to 
a particular height, this invariant is no longer necessary.

## Decision

Allow applications to start with a bootstrapped state alongside an empty Tendermint instance using
a subsystem of the state sync protocol. 

## Detailed Design

Tendermint will perform a protocol to bootstrap to the height of the application under the following circumstances:

- The returned application height is greater than Tendermint's blockstore height
- Tendermint's block store has a height of 0 (i.e. it's empty and we're not overriding an existing instance. This check may be relaxed in the future) 
- State sync is enabled and the config is valid (i.e it contains at least 2 rpc endpoints and a trusted header and hash)

The bootstrap protocol will be run directly after the handshake. It (currently) does not require the p2p layer and will work by updating the `StateStore` and `BlockStore`. It should be performed instead of statesync, moving to either blocksync or consensus once completed.

The protocol mainly wraps around the `StateProvider` which is derived from the `StateSyncConfig` and currently uses the RPC layer and light client to produce a `State` and `Commit` (to be pased to consensus for the next height)

The pseudocode is as follows:
```go
func Bootstrap(sp StateProvider, appHeight int64, appHash []byte, bs BlockStore, ss StateStore) (*State, error) {
    state, err := sp.State(appHeight)
    
    if !bytes.Equal(state.AppHash, appHash) {
        return errors.New("application state hash mismatches block's app hash")
    }

    lastCommit, err := sp.Commit(appHeight)

    err = ss.Bootstrap(state)

    err = bs.SaveSeenCommit(lastCommit)

    return state, err
}
```

## Consequences

> This section describes the consequences, after applying the decision. All
> consequences should be summarized here, not just the "positive" ones.

### Positive

- This is a non-breaking change
- Applications can now use their own state syncing protocols. This is especially useful when the saved snapshot is already from a trusted node.
- Tendermint will still verify application state in the event that the out of band state does not match the hash of the respective block on chain 

### Negative

### Neutral

- Users will need to have state sync correctly configured and enabled to take advantage of this functionality

## References

> Are there any relevant PR comments, issues that led up to this, or articles
> referenced for why we made the given design choice? If so link them here!

- [Initial Issue](https://github.com/tendermint/tendermint/issues/4642)
