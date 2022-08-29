# Switch

The `Switch` is documented in the [code](https://github.com/tendermint/tendermint/blob/46badfabd9d5491c78283a0ecdeb695e21785508/p2p/switch.go#L69) as follows:

> // Switch handles peer connections and exposes an API to receive incoming messages
> // on `Reactors`.

The switch is created from a configuration and a `Transport` implementation.

The switch implements a `BaseService` from `libs/service` package.

## Adddress book

`AddrBook` interface:

> // An AddrBook represents an address book from the pex package, which is used
> // to store peer addresses.

TODO: document the use of the address book both in the switch and in the PEX reactor.

## AddReactor

The reactor is associated to the set of channel ids it employs.
Two reactors (in the same node) cannot share the same channel id.

There is a call back to the reactor, in which the switch passes itself to the
reactor.

## RemoveReactor

The reactor is disassociated from the set of channel ids it employs.

There is a call back to the reactor, in which the switch passes `nil` to the
reactor.

## OnStart

This is a `BaseService` method.

All registered reactors are started.

The switch's `acceptRoutine` is started.

## OnStop

This is a `BaseService` method.

All (connected) peers are stopped and removed from the peer's list using the
`stopAndRemovePeer` method.

All registered reactors are stopped.

## StopPeerForError

Stop peer and remove it from the peer's table using the `stopAndRemovePeer` method.
The provided "reason" is passed to the method.

If the peer is a persistent peer, the method creates a routine to attempt
reconnecting to the peer, using the `reconnectToPeer` method.

This routine receives a `NetAddress` instance. If the peer is an outbound peer,
the peer's address is know, since the switch dialed the peer. Otherwise, the
node address is retrieved from its peer instance. This method can fail.

> Check when `peer.NodeInfo().NetAddress()` returns an error.

## StopPeerGracefully

Stop peer and remove it from the peer's table using the `stopAndRemovePeer` method.
No "reason" (nil) is passed to the method.

## stopAndRemovePeer

This method stops a peer instance, and remove the peer for all registered reactors.
The "reason" for which the peer was stopped is passed to the registered reactors.

The peer is then removed from the `peers` table, which is an instance of `PeerSet`.

> Issue https://github.com/tendermint/tendermint/issues/3338 is mentioned here,
> explaining why this action is the last taken.

## reconnectToPeer

The `reconnecting` synchronized map keeps the peer's in this state, identified
by their IDs (string).
This should ensure that a single instance of this method is running at any time.
The peer is kept in this map while this method is running for it: it is set on
the beginning, and removed when the method returns for whatever reason.
If the peer is already in the `reconnecting` state, nothing is done.

The remaining of the method performs multiple connection attempts to the peer,
via `DialPeerWithAddress` method.
If a connection attempt succeeds, the methods (and routine) returns (finishes).
If a connection attempt fails with `ErrCurrentlyDialingOrExistingAddress`
error, the switch gives up reconnecting to the peer.

> // ErrCurrentlyDialingOrExistingAddress indicates that we're currently
> // dialing this address or it belongs to an existing peer.

A first set of connection attempts is done at (about) regular intervals.
More precisely, between two attempts, the switch waits for a interval of
`reconnectInterval`, hard-coded to 5 seconds, plus a random jitter up to
`dialRandomizerIntervalMilliseconds`, hard-coded to 3 seconds.
At most `reconnectAttempts`, hard-coded to 20, are made using this
regular-interval approach.

A second set of connection attempts is done with exponentially increasing
intervals.
The base interval `reconnectBackOffBaseSeconds` is hard-coded to 3 seconds,
which is also the increasing factor.
The exponentially increasing dialing interval is adjusted as well by a random
jitter up to `dialRandomizerIntervalMilliseconds`.
At most `reconnectBackOffAttempts`, hard-coded to 10, are made using this
exponential-intervals approach.

> FIXME: the first sleep interval, to which a random jitter is applied, is 1,
> not `reconnectBackOffBaseSeconds`, as the first exponent is `0`...

## Dialing peers

Dialing a peer is implemented by the `DialPeerWithAddress` method.
It receives a peer address (`NetAddress` type), which also encodes the peer ID.

The switch keeps a single dialing routine per peer ID.
This is ensured by keeping a synchronized map `dialing` with the IDs of peers
to which the peer is dialing.
A peer ID is added to `dialing` when the `DialPeerWithAddress` method is called
for that peer, and it is removed when the method returns for whatever reason.
The method returns immediately when invoked for a peer which ID is already in
the `dialing` structure.

The actual dialing is implemented by the `Dial` method of the `Transport`
configured for the switch, invoked in the `addOutboundPeerWithConfig` method.
If the transport succeeds establishing a connection, the returned `Peer` is
added to the switch using the `addPeer` method.
This operation can fail, returning an error. In this case, the switch invokes
the `Transport.Cleanup` method to clean any resources associated with the peer.

> TODO: cleaning up a peer should mean, in practice, disconnecting from it.

If the transport fails to establish a connection with the peer, and the dialed
peer was configured as a persistent peer, the switch spawns a `reconnectToPeer`
routine for the peer.
If the peer is already in the `reconnecting` state, the spawned routine has no
effect and returns immediately.
This is in fact a likely scenario, as the `reconnectToPeer` routine relies on
this same `DialPeerWithAddress` method for dialing peers.

### `DialPeersAsync` 

This method receives a list of peer addresses (strings) and dials all of
them in parallel. 

> TODO: detail where this method is invoked:
>  - In the node setup, for every configured persistent peer
>  - In the rpc package, not in production as it is an unsafe RPC method

The list of peer address is parsed into `NetAddress` instances.
In case of parsing errors, the method returns. An exception is made for
`ErrNetAddressLookup` errors, which do not interrupt the procedure.

> TODO: when `ErrNetAddressLookup` errors are produced?

Before dialing the peers, they addresses are added to the switch's address
book, and changes to it are persisted to disk.

The switch dials to the multiple peers in parallel.
The list of peer addresses is randomly shuffled, and for each peer a routine is
spawned.
Each routine sleeps for a random interval, up to 3 seconds, then invokes the
`DialPeerWithAddress` method that actually dials the peer.

## Accepting peers

The `acceptRoutine` method is a persistent routine that handles connections
accepted by the transport configured for the switch.

The `Accept` method of the `Transport` returns a `Peer` with which an inbound
connection was established.
The maximum number of inbound peers, i.e., peers from which a connection was
accepted is determined by the `MaxNumInboundPeers` configuration parameter,
whose default value is `40`.
The switch accepts the peer if the maximum number of inbound peers was not
reached, or if the peer was configured as an _unconditional peer_.
If accepted, the peer is added to the switch using the `addPeer` method.

If the switch does not accept the established incoming connection, or if the
`addPeer` method returns an error, the switch invokes the `Transport.Cleanup`
method to clean any resources associated with the peer.

> TODO: cleaning up a peer should mean, in practice, disconnecting from it.

The transport's `Accept` method can also return a number of errors.
Errors of `ErrRejected` or `ErrFilterTimeout` types are ignored,
an `ErrTransportClosed` causes the accepted routine to be interrupted,
while other errors cause the routine to panic.

> TODO: ErrRejected has a multitude of causes.
> TODO: which source of errors can cause the routine to panic?

## addPeer

This method adds a peer to the switch,
either after dialing (by `addOutboundPeerWithConfig`, called by `DialPeerWithAddress`)
a peer and establishing an outbound connection,
or after accepting (`acceptRoutine`) a peer and establishing an inbound connection.

The first step is to invoke the `filterPeer` method.
It checks whether the peer is already in the set of connected peers,
and whether any of the configured `peerFilter` methods reject the peer.
If the peer is already present or it is rejected by any filter, the `addPeer`
method fails and returns an error.

Then, the new peer is started, added to the set of connected peers, and added
to all reactors
More precisely, first the new peer's information is first provided to every
reactor (`InitPeer` method).
Next, the peer's sending and receiving routines are started, and the peer is
added to set of connected peers.
These two operations can fail, causing `addPeer` to return an error.
Then, in the absence of previous errors, the peer is added to every reactor (`AddPeer` method).

> Adding the peer to the peer set returns a `ErrSwitchDuplicatePeerID` error
> when a peer with the same ID is already presented.
>
> TODO: Starting a peer could be reduced as starting the MConn with that peer?

## Broadcast

This method broadcasts a message on a channel, by sending the message in
parallel to all connected peers.

The method spawns a thread for each connected peer, invoking the `Send` method
provided by each `Peer` instance with the provided message and channel ID.
The return value (a boolean) of these calls are redirected to a channel that is
returned by the method.

> TODO: detail where this method is invocked:
> - By the consensus protocol, in `broadcastNewRoundStepMessage`,
>   `broadcastNewValidBlockMessage`, and `broadcastHasVoteMessage`
> - By the state sync protocol
