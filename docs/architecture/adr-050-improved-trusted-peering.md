# ADR 50: Improved Trusted Peering

## Changelog
* 22-10-2019: Initial draft
* 05-11-2019: Modify `maximum_dial_period` to `persistent_peers_max_dial_period`

## Context

When `max_num_inbound_peers` or `max_num_outbound_peers` of a node is reached, the node cannot spare more slots to any peer 
by inbound or outbound. Therefore, after a certain period of disconnection, any important peering can be lost indefinitely 
because all slots are consumed by other peers, and the node stops trying to dial the peer anymore.

This is happening because of two reasons, exponential backoff and absence of unconditional peering feature for trusted peers.


## Decision

We would like to suggest solving this problem by introducing two parameters in `config.toml`, `unconditional_peer_ids` and 
`persistent_peers_max_dial_period`. 

1) `unconditional_peer_ids`

A node operator inputs list of ids of peers which are allowed to be connected by both inbound or outbound regardless of 
`max_num_inbound_peers` or `max_num_outbound_peers` of user's node reached or not.

2) `persistent_peers_max_dial_period`

Terms between each dial to each persistent peer will not exceed `persistent_peers_max_dial_period` during exponential backoff. 
Therefore, `dial_period` = min(`persistent_peers_max_dial_period`, `exponential_backoff_dial_period`)

Alternative approach

Persistent_peers is only for outbound, therefore it is not enough to cover the full utility of `unconditional_peer_ids`. 
@creamers158(https://github.com/Creamers158) suggested putting id-only items into persistent_peers to be handled as 
`unconditional_peer_ids`, but it needs very complicated struct exception for different structure of items in persistent_peers.
Therefore we decided to have `unconditional_peer_ids` to independently cover this use-case.

## Status

Proposed

## Consequences

### Positive

A node operator can configure two new parameters in `config.toml` so that he/she can assure that tendermint will allow connections
from/to peers in `unconditional_peer_ids`. Also he/she can assure that every persistent peer will be dialed at least once in every 
`persistent_peers_max_dial_period` term. It achieves more stable and persistent peering for trusted peers.

### Negative

The new feature introduces two new parameters in `config.toml` which needs explanation for node operators.

### Neutral

## References

* two p2p feature enhancement proposal(https://github.com/tendermint/tendermint/issues/4053)
