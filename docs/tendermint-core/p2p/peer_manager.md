# Peer manager

The peer manager implements the connection policy for the node, based on the
configuration provided by the operators, the current state of the connections
(reported by the router), and the set of known candidate peers.

## Connection policy

The connection policy defines:

1. When the node should establish new connections to peers, and
1. The next peer to which the router should try to establish a connection.

The first definition is made based the concept of connection slots.
In short, the peer manager will try to fill every connection slot with a peer.

### Connection slots

The number of connection slots is defined by the `MaxConnected` parameter.

While there are available connection slots, the peer manager will provide
[candidate peers](#candidate-peer) to the router, which will try to establish
new connections to them.
When the peer manager [provides a candidate peer](#dialnext-transition) to
the router, a connection slot becomes _virtually_ occupied by the peer, as the
router should be [dialing](#dialing-peer) it.

When the router establishes a connection to a peer, either
because it [accepted a connection](#accepted-transition) from a peer,
or because it [successfully dialed](#dialed-transition) a candidate peer,
the peer manager should find a slot for this connection.

If there is an available connection slot, and this is the first connection
established with that peer, the slot is filled by the new connection and
the peer becomes a [connected peer](#connected-peer).
The peer manager does not allow two slots to be filled with connections to the
same peer.

If all `MaxConnected` connection slots are full, the node should _a priori_
reject the connection with the peer.
However, it is possible that the new connection is with a peer whose score is
better than the score of a peer occupying one of the connection slots.
In this case, the peer manager will try to [upgrade the slot](#slot-upgrades)
to make room to the new connection, by evicting the peer currently occupying
this slot.

> Although not advisable, the `MaxConnected` parameter can be set `0`.
> In this case, there is not limit in the number of connections a node can
> establish with peers.
> This means that the node will accept all connections established with peers,
> and will dial every candidate peer it knows about.

### Outgoing connections

The peer manager distinguishes *incoming* from *outgoing* connections.
A connection is *incoming* when the router has [accepted](#accepted-transition)
it from a peer.
A connection is *outgoing* when the router has successfully
[dialed](#dialed-transition) a peer.

If the `MaxOutgoingConnections` parameter is set (it is larger than zero), it
defines the maximum number of *outgoing* connections the node should maintain.
More precisely, it determines that the node should not attempt to dial new
peers when the router already has established outgoing connections to
`MaxOutgoingConnections` peers.

> The previous version of the `p2p` explicitly distinguished incoming and
> outgoing peers. Configuring the `MaxOutgoingConnections` parameters should
> therefore make the connection policy similar to the one adopted in the
> previous version. (TODO: check)

### Slot upgrades

The rationale behind this concept is that the node may try to establish or
accept connections even when all the connection slots are full, provided that
the peer in the other side of the new connection is better-ranked than a peer
that is occupying a connection slot.
A connection slot can therefore be upgraded, meaning that the lower-ranked peer
occupying this connection slot will be replaced by a higher-ranked peer.

The upgrading of connection slots is determined by the `MaxConnectedUpgrade`
parameter, which defines the number of connections that the peer manager can
use for upgrading connection slots.

If `MaxConnectedUpgrade` is set to zero, the upgrading of connection slots is
disabled.
This means, in particular, that `MaxConnected` is the hard limit of peers that
can be in the [connected state](#connected-peer).

If `MaxConnectedUpgrade` is larger than zero, the upgrading of connection slots
is enabled.
As a result, the hard limit for the number of peers that can be in the
[connected state](#connected-peer) becomes `MaxConnected + MaxConnectedUpgrade`.
Some of these peers, however, will not remain in this state as they should be
[evicted](#evictnext-transition) by the router.

### Peer ranking

TODO:

## Peer life cycle

For implementing the connection policy, the peer manager keeps track of the
state of peers and manages their life-cycle, using a `peerStore` for underlying
storage.

The life cycle of a peer in the peer manager is represented in the picture
below.
The circles represent _states_ of a peer and the rectangles represent
_transitions_.
All transitions are performed by the `Router`, by invoking methods with
corresponding names.
Green arrows represent normal transitions, while red arrows represent
alternative transitions taken in case of errors.

<img src="pics/p2p-v0.35-peermanager.png" alt="peer life cycle" title="" width="600px" name="" align="center"/>

### Candidate peer

The initial state of a peer in the peer manager.

This document uses *candidate peer* to refer to the information about a node in
the network.
This information can be manually configured by the node operator (e.g., via
`PersistentPeers` parameter) or can be obtained via the PEX protocol.

A *candidate peer* may become an actual peer, to which the node is connected.
We do not use *candidate* to refer to a peer to which we are connected, nor to
a peer we are attempting to connect.

Candidate peers from which the router recently disconnected or failed to dial
are not eligible for establishing connections.
This scenario is represented by the `Frozen Candidate` state.

### DialNext transition

This transition is performed when the [connection policy](#connection-policy)
determines that the node should try to establish a connection with a peer, and
there are peers available in the [`Candidate`](#candidate-peer) state.

When both conditions are met, the peer manager selects the
[best-ranked](#peer-ranking) candidate peer and provides it to the router,
which is responsible for dialing the peer.

Dialing this candidate peer may have become possible because the peer manager
has found a connection slot to [upgrade](#slots-upgrades) for (possibly) given
room to the selected candidate peer.
If this is the case, the peer occupying this connection slot is set to the
[upgrading state](#upgrading-peer), so that it will be evicted once the
connection to the candidate peer is successfully established.

### Dialing peer

A peer that was returned to the router as the next peer to dial.

While the router is attempting to connect to the peer, it is not considered as
a candidate peer.

### Dialed transition

This transition is performed when the node establishes an *outgoing* connection
with a peer.
The router has dialed and successfully established a connection with the peer.

This peer should be a candidate peer that has been provided to the router,
i.e., it should be in the [`Dialing`](#dialing-peer) state.
It may occur, however, that this peer is already in the
[`Connected`](#connected-peer) state.
This can happen because the router already successfully dialed or accepted a
connection from the same peer.
In this case, the transitions fails.

> Question: is it possible to have multiple routines dialing to the same peer?

It may also occur that the node is already connected to `MaxConnected` peers,
which means that all connection slots are full.
In this case, the peer manager tries to find a connection slot that can be
[upgraded](#slots-upgrading) to give room for the new established connection.
If another peer was set to the [upgrading state](#upgrading-peeer) to give room
to this peer, the slot for the established connection is reserved.
Otherwise, if no suitable connection slot is found, or the hard limit
`MaxConnected + MaxConnectedUpgrade` of connected peers is reached, the
transitions fails.

If the transition succeeds, the peer is set to the
[`Connected`](#connected-peer) state as an `outgoing` peer.
The peer's `LastConnected` and the dialed address' `LastDialSuccess` times are
set, and dialed address' `DialFailures` counter is reset.

> If the peer is `Inactive`, it is set as active.
> This action has no effect apart from producing metrics.

If a connection slot was upgraded to give room for the established connection, the
peer occupying that slot transitions to the [evict state](#evict-peer).

> The peer to evict could have been previously selected in the associated
> [next to dial transition](#dialnext-transition).
> In this case, the peer manager looks for an even lower-ranked peer to replace
> the peer that was previously selected; if such a peer is found, it is is to
> be evicted, not the originally selected.

#### Errors

The transition fails if:

- the node dialed itself
- the peer is already in the `Connected` state
- the node is connected to enough peers, and eviction is not possible

Errors are also returned if:

- the dialed peer was removed from the peer store
- the updated peer information is invalid
- there is an error when saving the peer state in the peer store

In either case, the router closes the established connection with the peer.

### DialFailed transition

This transition informs a failure when establishing an `outgoing` connection to
a peer.

The dialed address's `LastDialFailure` time is set, and its `DialFailures`
counter is increased.
This information is used to compute the retry delay for the dialed address, as
detailed below.

The peer manager then spawns a routine that after the computed retry delay
notifies the next peer to dial routine about the availability of this peer.
Until then, the peer is the `Frozen Candidate` state.

#### Retry delay

The retry dial is the minimum time, from the latest failed dialing attempt, we
should wait until dialing a peer address again.

The default delay is defined by `MinRetryTime` parameter.
If it is set to zero, we *never* retry dialing a peer address.

Upon each failed dial attempt, we increase the delay by `MinRetryTime`, plus an
optional random jitter of up to `RetryTimeJitter`.

The retry delay should not be longer than the `MaxRetryTime` parameter.
For *persistent* peers, a different `MaxRetryTimePersistent` can be set.

> This is a linear backoff, while the code mentions an exponential backoff.

#### Errors

Errors are also returned if:

- the updated peer information is invalid
- there is an error when saving the peer state in the peer store

### Accepted transition

This transition is performed when the node establishes an *incoming* connection
with a peer.
The router has accepted a connection from and successfully established a
connection with the peer.

It may occur, however, that this peer is already in the
[`Connected`](#connected-peer) state.
This can happen because the router already successfully dialed or accepted a
connection from the same peer.
In this case, the transitions fails.

It may also occur that the node is already connected to `MaxConnected` peers,
which means that all connection slots are full.
In this case, the peer manager tries to find a connection slot that can be
[upgraded](#slots-upgrading) to give room for the accepted connection.
If no suitable connection slot is found, or the hard limit `MaxConnected +
MaxConnectedUpgrade` of connected peers is reached, the transitions fails.

If the transition succeeds, the peer is set to the
[`Connected`](#connected-peer) state as an `incoming` peer.

The accepted peer might not be known by the peer manager.
In this case it is registered in the peer store, without any associated
address.
The peer `LastConnected` time is set and the `DialFailures` counter is reset
for all addresses associated to the peer.

> If the peer is `Inactive`, it is set as active.
> This action has not effect apart from producing metrics.

If a connection slot was upgraded to give room for the accepted connection, the
peer occupying that slot transitions to the [evict state](#evict-peer).

#### Errors

The transition fails if:

- the node accepted itself
- the peer is already in the `Connected` state
- the node is connected to enough peers, and eviction is not possible

Errors are also returned if:

- the updated peer information is invalid
- there is an error when saving the peer state in the peer store

In either case, the router closes the established connection with the peer.

### Connected peer

A peer to which the node is connected.
A peer in this state is not considered a candidate peer.

The peer manager distinguishes *incoming* from *outgoing* connections.
Incoming connections are established through the [`Accepted`](#accepted-transition) transition.
Outgoing connections are established through the [`Dialed`](#dialed-transition) transition.

### Ready transition

This transition is not represented in the picture because it does not change
the state of the peer.
It notifies the peer manager that the node is connected and can exchange
messages with a peer.

The router performs this transition just after successfully performing the
[`Dialed`](#dialed-transition) or [`Accepted`](#accepted-transition) transitions.
It provides to the peer manager a list of channels supported by this peer,
information which is broadcast to all subscriptions a `PeerUpdate` message that
also informs the new state of the peer (up).

### Disconnected transition

This transition is performed when the node stops exchanging messages with a
peer, due to an error in the peer's message sending or receiving routines.

The peer is expected to be in the [`Connected`](#connected-peer) state.
If the [`Ready`](#ready-transition) transition has been performed, the peer manager broadcast a
`PeerUpdate` to all subscriptions notifying the new status (down) of this peer.

If the peer is still present in the peer store, its `LastDisconnected` time is
set and the peer manager spawns a routine that after `DisconnectCooldownPeriod`
notifies the next peer to dial routine about the availability of this peer.
Until then, the peer is the `Frozen Candidate` state.

### Errored transition

This transition is performed when a reactor interacting with the peer reports
an error to the router.

The peer is expected to be in the [`Connected`](#connected-peer) state.
If so, the peer transitions to the [`Evict`](#evict-peer) state, which should lead the router
to disconnect from the peer, and the next peer to evict routine is notified.

### Upgrading peer

A peer whose eviction was scheduled to given room to a higher-ranked peer the
router is dialing to.

This is a [connected peer](#connected-peer), and the `Upgrading` sub-state is
part of the connection [slots upgrading](#slots-upgrade) procedure.

The eviction of this peer, i.e. its transition to the [evict](#evict-peer)
sub-state, only occur if and when a connection with the higher-ranker peer that
should take its connection slot is [established](#dialed-transition).

### Evict peer

### EvictNext transition

### Evicting peer

[peermanager.go]: https://github.com/tendermint/tendermint/blob/v0.35.x/internal/p2p/peermanager.go
