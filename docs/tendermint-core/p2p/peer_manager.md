# Peer manager

The [`PeerManager`][peermanager.go] manages peer life-cycle information, using
a `peerStore` for underlying storage.

## Next peer to dial

This is the main service provided by the peer manager.
The peer manager should select the best-ranked *candidate peer* to which the
node should establish a connection, and provide it to the router.

By *candidate peer* we mean the information about a node in the network.
This information can be manually configured by the node operator (e.g., via
`PersistentPeers` parameter) or can be obtained via PEX protocol.

A *candidate peer* may become an actual peer, to which the node is connected.
We do not use *candidate* to refer to a peer to which we are connected, nor to
a peer we are attempting to connect.
The state of a peer (connected, attempting to connect, disconnected) is
reported to the peer manager by the router.

The peer manager keeps track of the state of peers and is responsible for
defining when the node is connected to enough peers.
At this point, and while its state does not change (e.g., a peer is
disconnected), it should not provide the router with *candidate peers*.

The peer manager therefore implements the connection `policy` for the node,
based on the configuration provided by the operators, the current state of the
connections, and the set of known candidate peers.

### Policy

The peer manager **does not** return a candidate peer if:

1. `|connected + dialing| >= MaxConnected + MaxConnectedUpgrade`
1. `|connected.outgoing| >= MaxOutgoingConnections > 0`

The peer manager **may not** return a candidate peer if:

1. `|connected| >= MaxConnected`, provided we cannot find a low-rank connected
   peer to evict
1. We have recently disconnected from our candidate peers, provided that the
   `DisconnectCooldownPeriod` has not yet passed
1. We have recently failed to dial our candidate peers, provided that the
   retry delay for all addresses of every peer has not yet passed

Observations:

- `MaxConnected` in thesis can be set to `0`, meaning there are no limits for
  number of connections established;
- If `MaxConnectedUpgrade` is set to `0`, `MaxConnected` becomes a hard limit;
- `MaxOutgoingConnections` can be set to `0`, meaning that the node should not
  make distinction between *incoming* and *outgoing* connections.

#### Dial retry delay

For each network address of a peer, the peer manager keeps the number of failed
dial attempts and the time of the latest failed dial attempt.

The retry dial is the minimum time, from the latest failed dial attempt, we
should wait until dialing a peer address again.

The default delay is defined by `MinRetryTime` parameter.
If it is set to zero, we *never* retry dialing a peer address.

Upon each failed dial attempt, we increase the delay by `MinRetryTime`, plus an
optional random jitter of up to `RetryTimeJitter`.

The retry delay should not be longer than `MaxRetryTime` parameter.
For *persistent* peers, a different `MaxRetryTimePersistent` should be set.

> This is a linear backoff, while the code mentions an exponential backoff.

### Interaction with the router

The `Router` invokes the `PeerManager.DialNext()` method in a closed loop to
obtain candidate peers to which dial, in the form of a `NodeAddress` object.
This methods blocks when there are no candidate peers or the peer manager
decides that the node has established enough connections.

[peermanager.go]: https://github.com/tendermint/tendermint/blob/v0.35.x/internal/p2p/peermanager.go

