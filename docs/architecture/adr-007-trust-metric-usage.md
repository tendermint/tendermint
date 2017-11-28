# ADR 007: Trust Metric Usage Guide

## Context

The Tendermint project developers would like to improve Tendermint security and reliability by keeping track of the quality that peers have demonstrated. This way, undesirable outcomes from peers will not immediately result in them being dropped from the network (potentially causing drastic changes). Instead, a peer's behavior can be monitored with appropriate metrics and can be removed from the network once Tendermint is certain the peer is a threat. For example, when the PEXReactor makes a request for peers network addresses from an already known peer, and the returned network addresses are unreachable, this undesirable behavior should be tracked. Returning a few bad network addresses probably shouldn’t cause a peer to be dropped, while excessive amounts of this behavior does qualify the peer for removal. The originally proposed approach and design document for the trust metric can be found in the [ADR 006](adr-006-trust-metric.md) document.

The trust metric implementation allows a developer to obtain a peer's trust metric from a trust metric store, and track good and bad events relevant to a peer's behavior, and at any time, the peer's metric can be queried for a current trust value. The current trust value is calculated with a formula that utilizes current behavior, previous behavior, and change between the two. Current behavior is calculated as the percentage of good behavior within a time interval. The time interval is short; probably set between 30 seconds and 5 minutes. On the other hand, the historic data can estimate a peer's behavior over days worth of tracking. At the end of a time interval, the current behavior becomes part of the historic data, and a new time interval begins with the good and bad counters reset to zero.

These are some important things to keep in mind regarding how the trust metrics handle time intervals and scoring:
- Each new time interval begins with a perfect score
- Bad events quickly bring the score down and good events cause the score to slowly rise
- When the time interval is over, the percentage of good events becomes historic data.

Some useful information about the inner workings of the trust metric:
- When a trust metric is first instantiated, a timer (ticker) periodically fires in order to handle transitions between trust metric time intervals
- If a peer is disconnected from a node, the timer should be paused, since the node is no longer connected to that peer
- The ability to pause the metric is supported with the store **PeerDisconnected** method and the metric **Pause** method
- After a pause, if a good or bad event method is called on a metric, it automatically becomes unpaused and begins a new time interval.

## Decision

The trust metric capability is now available, yet, it still leaves the question of how should it be applied throughout Tendermint in order to properly track the quality of peers? 

### Proposed Process

Peers are managed using an address book and a trust metric:

- The address book keeps a record of peers and provides selection methods
- The trust metric tracks the quality of the peers

When we need more peers, we pick them randomly from the address book's selection method. When we're asked for peers, we provide a random selection with no bias:

- The address book's selection method will perform peer ranking based on trust metric scores
- If we need to make room for a new peer, we remove the peer with the lowest trust metric score

Peer quality is tracked in the connection and across the reactors, and behaviors are defined as one of the following:
- Fatal - something outright malicious that causes us to disconnect the peer and remember it
- Bad - Any kind of timeout, messages that don't unmarshal, fail other validity checks, or messages we didn't ask for or aren't expecting (usually worth one bad event)
- Neutral - Unknown channels/message types/version upgrades (no good or bad events recorded)
- Correct - Normal correct behavior (worth one good event)
- Good - some random majority of peers per reactor sending us useful messages (worth more than one good event).

## Status

Proposed.

## Consequences

### Positive

- Bringing the address book and trust metric store together will cause the network to be built in a way that encourages greater security and reliability.

### Negative

- TBD

### Neutral

- Keep in mind that, good events need to be recorded just as bad events do using this implementation.
