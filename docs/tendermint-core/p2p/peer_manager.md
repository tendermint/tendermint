# Peer manager

The [`PeerManager`][peermanager.go] manages peer life-cycle information, using
a `peerStore` for underlying storage.

## Peer life-cycle

<img src="pics/p2p-v0.35-peermanager.png" alt="peer life cycle" title="" width="400px" name="" align="center"/>

### Dialing

A candidate peer is returned to the router as the next peer to dial.

### Connected

A peer to which the node is connected.
A peer in this state is not considered a candidate peer.

The peer manager distinguishes *incoming* from *outgoing* connections.
Incoming connections are established through the `Accepted` transition.
Outgoing connections are established through the `Dialed` transition.

### Ready

A peer that is connected and with which the node can exchange messages.

The router sets a peer to this state just after successfully setting the peer
as the `Dialed` or `Accepted`.
The reason (from comments in the code), is to allow the router to setup
internal queues to interact with the peer and reactors.

In fact, the router provides to the peer manager a list of channels
(communication abstraction used by reactors) to be informed to all reactors.
When entering this state, the peer manager broadcasts a `PeerUpdate` to all
subscriptions, notifying the new state (up) of this peer and providing the list
of channels.

### Dialed transition

This transition is performed when the node establishes an *outgoing* connection
with a peer.
The router has dialed and successfully established a connection with the peer.

This peer should be a candidate peer that has been provided to the router.
So, this node should be in `dialing` state, from which it is removed.

It may occur, however, that this peer is already in `connected` state.
This can happen because the router already successfully dialed or accepted a
connection from the same peer.
In this case, the transitions fails.

> Question: is it possible to have multiple dialing routines to the same peer?

It may also occur that the node is already connected to `MaxConnected` peers.
In this case, the peer manager has to find a peer to evict to give its place to
the newly established connection.
If a non suitable connected peer is found for eviction, or the hard limit of
connected peers (`MaxConnected + MaxConnectedUpgrade`) is reached, the
transitions fails.

If the transition succeeds, the peer is set to the `connected` state as an
*outgoing* peer.
The peer `LastConnected` time is set, the dialed address's `LastDialSuccess`
time is set, and its `DialFailures` counter is reset.

> If the peer is `Inactive`, it is set as active.
> This action has no effect apart from producing metrics.

#### Errors

The transition fails if:

- node dialed itself
- peer is already in `connected` state
- node is connected to enough peers, and eviction is not possible

Errors are also returned if:

- dialed peer was removed from the peer store
- the peer information is invalid
- unable to save peer state in the store

Result: the router closes the established connection with the peer.

#### Eviction procedure

TODO: detail this eviction procedure

### DialFailed transition

This transition informs a failure when establishing an *outgoing* connection to
a peer.

This peer should be a candidate peer that has been provided to the router.
So, this node should be in `dialing` state, from which it is removed.

The dialed address's `LastDialFailure` time is set, and its `DialFailures`
counter is increased.
This information is used to compute the retry delay for the peer.

The goal of this transition is to restore the peer to a candidate state.
The peer manager then spawns a routine that after the computed retry delay
notifies the next peer to dial routine about the availability of this peer.

#### Errors

Errors are also returned if:

- peer information is invalid
- unable to save peer state in the store

When the router informs this transition, dialing the peer already failed.

### Accepted transition

This transition is performed when the node establishes an *incoming* connection
with a peer.
The router has accepted a connection from and successfully established a
connection with the peer.

It may occur, however, that this peer is already in `connected` state.
This can happen because the router already successfully dialed or accepted a
connection from the same peer.
In this case, the transitions fails.

It may also occur that the node is already connected to `MaxConnected` peers.
In this case, the peer manager has to find a peer to evict to given place to
the newly established connection.
If no suitable connected peer is found for eviction, or the hard limit of
connected peers (`MaxConnected + MaxConnectedUpgrade`) is reached, the
transitions fails.

If the transition succeeds, the peer is set to the `connected` state as an
*incoming* peer.

The accepted peer might not be known by the peer manager.
In this case it is registered in the peer store, without any associated
address.
The peer `LastConnected` time is set and the `DialFailures` counter is reset
for all addresses associated to the peer.

> If the peer is `Inactive`, it is set as active.
> This action has not effect apart from producing metrics.

#### Errors

The transition fails if:

- node accepted itself
- peer is already in `connected` state
- node is connected to enough peers, and eviction is not possible

Errors are also returned if:

- peer information is invalid
- unable to save peer state in the store

Result: the router closes the established connection with the peer.

#### Eviction procedure

TODO: detail this eviction procedure

### Disconnected transition

A peer with which the node is not exchanging messages any longer.

The peer is expected to be in `ready` state.
In this case, the peer manager broadcast a `PeerUpdate` to all subscriptions
notifying the new status (down) of this peer.

Once disconnected, the peer should become a candidate peer.
The peer manager then spawns a routine that after `DisconnectCooldownPeriod`
notifies the next peer to dial routine about the availability of this peer.

## Next peer to dial

This is the main service provided by the peer manager.
The peer manager should select the best-ranked *candidate peer* to which the
node should establish a connection, and provide it to the router.

By *candidate peer* we mean the information about a node in the network.
This information can be manually configured by the node operator (e.g., via
`PersistentPeers` parameter) or can be obtained via the PEX protocol.

A *candidate peer* may become an actual peer, to which the node is connected.
We do not use *candidate* to refer to a peer to which we are connected, nor to
a peer we are attempting to connect.
The state of a peer (connected, attempting to connect, disconnected) is
reported to the peer manager by the router.

The peer manager keeps track of the state of peers and is responsible for
defining when the node is connected to enough peers.
At this point, and while its state does not change (e.g., a peer is
disconnected), it should not provide the router with *candidate peers*.

The peer manager therefore implements the connection *policy* for the node,
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

- `MaxConnected` in thesis(theory?) can be set to `0`, meaning there are no limits for the
  number of connections established;
- If `MaxConnectedUpgrade` is set to `0`, `MaxConnected` becomes a hard limit;
- `MaxOutgoingConnections` can be set to `0`, meaning that the node should not
  make distinction between *incoming* and *outgoing* connections.

#### Dial retry delay

For each network address of a peer, the peer manager keeps the number of failed
dial attempts and the time of the latest failed dial attempt.

The retry dial is the minimum time, from the latest failed dialing attempt, we
should wait until dialing a peer address again.

The default delay is defined by `MinRetryTime` parameter.
If it is set to zero, we *never* retry dialing a peer address.

Upon each failed dial attempt, we increase the delay by `MinRetryTime`, plus an
optional random jitter of up to `RetryTimeJitter`.

The retry delay should not be longer than the `MaxRetryTime` parameter.
For *persistent* peers, a different `MaxRetryTimePersistent` should be set.

> This is a linear backoff, while the code mentions an exponential backoff.

### Interaction with the router

The `Router` invokes the `PeerManager.DialNext()` method in a closed loop to
obtain candidate peers which to dial, in the form of a `NodeAddress` object.
This method blocks when there are no candidate peers or the peer manager
decides that the node has established enough connections.

[peermanager.go]: https://github.com/tendermint/tendermint/blob/v0.35.x/internal/p2p/peermanager.go

