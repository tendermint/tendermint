# Router - WIP

The router is the component of the *new* p2p layer
responsible for establishing connection with peers,
and for routing messages from reactors to peers and vice-versa.

## Dialing peers

The router maintains a persistent routine `dialPeers()` consuming
[candidate peers to dial](./peer_manager.md#dialnext-transition)
produced by the peer manager.

The consumed candidate peers (addresses) are provided for dialing routines,
retrieved from a pool with `numConcurrentDials()` threads.
The default number of threads in the pool is set to 32 times the number of
available CPUs.

> The 32 factor was introduced in [#8827](https://github.com/tendermint/tendermint/pull/8827),
> with the goal of speeding up the establishment of outbound connections.

The router thus dials a peer whenever there are: (i) a candidate peer to be
consumed and (ii) a dialing routine is available in the pool.
Given the size of the thread pool, the router is in practice is expected to
dial in parallel all candidate peers produced by the peer manager.

> There was a random-interval sleep between starting subsequent dialing
> routines. This behavior was removed by [#8839](https://github.com/tendermint/tendermint/pull/8839).

The dialing routine selected to dial to a peer runs by the `connectPeer()`
method, which:

1. Calls `dialPeer()` to establish a connection with the remote peer
   1. In case of errors, invokes the `DialFailed` transition of the peer manager
1. Calls `handshakePeer()` with the established connection and the expected remote node ID
   1. In case of errors, invokes the `DialFailed` transition of the peer manager
1. Reports the established outbound connection through the `Dialed` transition of the peer manager
   1. In the transition fails, the established connection was refused
1. Spawns a `routePeer()` routine for the peer

> Step 3. above acquires a mutex, preventing concurrent calls from different threads.
> The reason is not clear, as all peer manager transitions are also protected by a mutex.
>
> Step 3i. above also notifies the peer manager's next peer to dial routine.
> This should trigger the peer manager to produce another candidate peer.
> TODO: check when this was introduced, as it breaks modularity.

In case of any of the above errors, the connection with the remote peer is
**closed** and the dialing routines returns.

## Accepting peers

The router maintains a persistent routine `acceptPeers()` consuming connections
accepted by each of the configured transports.

Each accepted connection is handled by a different `openConnection()` routine,
spawned for this purpose, that operate as follows.
There is no limit for the number of concurrent routines accepting peer's connections.

1. Calls `filterPeersIP()` with the peer address
   1. If the peer IP is rejected, the method returns
1. Calls `handshakePeer()` with the accepted connection to retrieve the remote node ID
   1. If the handshake fails, the method returns
1. Calls `filterPeersID()` with the peer ID (learned from the handshake)
   1. If the peer ID is rejected, the method returns
1. Reports the established incoming connection through the `Accepted` transition of the peer manager
   1. In the transition fails, the accepted connection was refused and the method returns
1. Switches to the `routePeer()` routine for the accepted peer

> Step 4. above acquires a mutex, preventing concurrent calls from different threads.
> The reason is not clear, as all peer manager transitions are also protected by a mutex.

In case of any of the above errors, the connection with the remote peer is
**closed**.

> TODO: Step 2. above has a limitation, commented in the source code, referring
> to absence of an ack/nack in the handshake, which may case further
> connections to be rejected.

> TODO: there is a `connTracker` in the router that rate limits addresses that
> try to establish connections to often. This procedure should be documented.

## Evicting peers

The router maintains a persistent routine `evictPeers()` consuming
[peers to evict](./peer_manager.md#evictnext-transition)
produced by the peer manager.

The eviction of a peer is performed by closing the send queue associated to the peer.
This queue maintains outbound messages destined to the peer, consumed by the
peer's send routine.
When the peer's send queue is closed, the peer's send routine is interrupted
with no errors.

When the [routing messages](#routing-messages) routine notices that the peer's
send routine was interrupted, it forces the interruption of peer's receive routine.
When both send and receive routines are interrupted, the router considers the
peer as disconnected, and its eviction has been done.

## Routing messages

When the router successfully establishes a connection with a peer, because it
dialed the peer or accepted a connection from the peer, it starts routing
messages from and to the peer.

This role is implemented by the `routePeer()` routine.
Initially, the router notifies the peer manager that the peer is
[`Ready`](./peer_manager.md#ready-transition).
This notification includes the list of channels IDs supported by the peer,
information obtained during the handshake process.

Then, the peer's send and receive routines are spawned.
The send routine receives the peer ID, the established connection, and a new
send queue associated with the peer.
The peer's send queue is fed with messages produced by reactors and destined to
the peer, which are sent to the peer through the established connection.
The receive routine receives the peer ID and the established connection.
Messages received through the established connections are forwarded to the
appropriate reactors, using message queues associated to each channel ID.

From this point, the routing routine will monitor the peer's send and receive routines.
When one of them returns, due to errors or because it was interrupted, the
router forces the interruption of the other.
To force the interruption of the send routine, the router closes the peer's
send queue. To force the interruption of the receive routine, the router closes
the connection established with the peer.

Finally, when both peer's send and receive routine return, the router notifies
the peer manager that the peer is [`Disconnected`](./peer_manager.md#disconnected-transition).
