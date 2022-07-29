# Transport

The transport establishes secure and authenticated connections with peers.

It [`Dial`](#dialing-peers) peers to establish outbound connections,
and [`Listen`](#listen) in a configured network address
to [`Accept`](#accepting-peers) inbound connections from peers.

The transport establishes raw TCP connections with peers
and  [upgrade](#connection-upgrade) them into authenticated secret connections.
The established secret connection is then wrapped into `Peer` instance, which
is returned to the caller, typically the [switch](./switch.md).

## Dialing peers

The `Dial` method is used by the switch to establish an outbound connection with a peer.
It is a synchronous method, which blocks until a connection is established or an error occurs.
The method returns an outbound `Peer` instance wrapping the established connection.

The transport first dials the provided peer's address to establish a raw TCP connection.
The dialing maximum duration is determined by `dialTimeout`, hard-coded to 1 second.
The established raw connection is then submitted to a set of [filters](#filtering-connections),
which can reject it.
If the connection is not rejected, it is recorded in the table of established connections.

The established raw TCP connection is then [upgraded](#connection-upgrade) into
an authenticated secret connection.
This procedure should ensure, in particular, that the public key of the remote peer
matches the ID of the dialed peer, which is part of peer address provided to this method.
In the absence of errors,
the established secret connection (`conn.SecretConnection` type)
and the information about the peer (`NodeInfo` record) retrieved and verified
during the version handshake,
are wrapped into an outbound `Peer` instance and returned to the switch.

The dialing timeout is configured to 1 second.
The filter connection timeout is configured to 3 seconds, and can be changed
by the `MultiplexTransportFilterTimeout` transport option.
The handshake timeout is configured to 5 seconds.
Hitting some of these timeouts causes an error, returned to the switch.

## Accepting peers

The `Accept` method returns to the switch inbound connections established with a peer.
It is a synchronous method, which blocks until a connection is accepted or an error occurs.
The method returns an inbound `Peer` instance wrapping the established connection.

The transport handles incoming connections in the `acceptPeers` persistent routine.
This routine is started by the `Listen` method and accepts raw connections from a TCP listener.
A new routine is spawned for each accepted connection.
The raw connection is submitted to a set of [filters](#filtering-connections),
which can reject it.
If the connection is not rejected, it is recorded in the table of established connections.

The established raw TCP connection is then [upgraded](#connection-upgrade) into
an authenticated secret connection.
The established secret connection (`conn.SecretConnection` type),
the information about the peer (`NodeInfo` record) retrieved and verified
during the version handshake,
as well any error returned in this process are added to a queue of accepted connections.
This queue is consumed by the `Accept` method.

> Handling connection asynchronously was introduced due to this issue:
> https://github.com/tendermint/tendermint/issues/2047

## Filtering Connections

The `filterConn` method is invoked for every new raw connection established by the transport.
Its main goal is avoid the transport to maintain duplicated connections with the same peer.
It also allows a set of filters to be configured,
using the `MultiplexTransportConnFilters` option.

The transports keeps a table `conns` of established connections,
indexed by the connection's remote address.
In the case of TCP connections, adopted by the transport,
the remote address is a string with the remote address (IP or DNS name) and port.
If the remote address of the new connection is already present in the table,
the method returns an `ErrRejected` error with the `isDuplicate` reason.

Each entry of the table of established connections contains a list of IPs
to which the connection's remote address is resolved.
The IP resolution is performed by the filtering method.
If it fails, an `AddrError` or `DNSError` error is returned.

The connection and the resolved IPs are then passed through a set of filters,
configured via the `MultiplexTransportConnFilters` transport option.
If any of the filters reject the new connection, 
the method returns an `ErrRejected` error with the `isRejected` reason.
If any of the filters execution times out,
the method returns an `ErrFilterTimeout` error.
The standard value of the filtering timeout is 5 seconds,
which can be changed using the `MultiplexTransportFilterTimeout` transport option.

If the connection and the resolved remote addresses are not filtered out,
the transport registers them into the `conns` table and returns.

## Connection Upgrade

The `upgrade` method is invoked for every new raw connection established by the
transport, that was not [filtered out](#filtering-connections).
It upgrades an established raw TCP connection into a secret authenticated
connection, an instance of `conn.SecretConnection` type.

This is a complex procedure, that can be summarized by the following three
message exchanges between the node and the new peer:

1. Encryption: the nodes produce ephemeral key pairs and exchange ephemeral
   public keys, from which are derived secret keys used to encrypt the data
   exchanged between the nodes, and a challenge message.
1. Authentication: the nodes exchange their persistent public keys, which also
   play the role of node IDs, plus a signature of the challenge message produced
   with the corresponding persistent private keys.
1. Version handshake: nodes exchange their `NodeInfo` records, containing,
   among other fields, their node IDs, the network/chain ID they are part of,
   and the list of supported channel IDs.

Steps (1) and (2) are implemented in the `conn` package.
In case of success, they produce the secret connection that is actually used by
the node to communicate with the peer.
An overview of this procedure, which implements the station-to-station (STS)
[protocol][sts-paper] ([PDF][sts-paper-pdf]), can be found [here][peer-sts].

The maximum duration for establishing a secret connection with the peer is
defined by `handshakeTimeout`, hard-coded to 3 seconds.
This duration is configured as the deadline for operations on the TCP
connection that is being upgraded; if this deadline it is reached, the
connection produces an `os.ErrDeadlineExceeded` error.
This error, or any error when establishing a secret connection,
is returned encapsulated into an `ErrRejected` error with reason `isAuthFailure`.

The established secret connection contains the public key of the remote peer,
i.e., its node ID.
If the connection being upgraded is an outbound connection, i.e., if the node
dialed the peer, the dialed peer's ID is compared to the node ID learned in step 2.
If they do not match, an `ErrRejected` error with reason `isAuthFailure` is returned.
In case of inbound (accepted) connections, the node does not know a priori the peer ID,
and this verification is not performed.

Step (3), the version handshake, is performed by the transport.
Its maximum duration is defined by `handshakeTimeout`, hard-coded to 3 seconds.
This duration is configured as the deadline for operations on the TCP
connection that is being upgraded; if this deadline it is reached, the
connection produces an `os.ErrDeadlineExceeded` error.
This error, or any error in the version handshake,
is returned encapsulated into an `ErrRejected` error with reason `isAuthFailure`.

The version handshake retrieves the `NodeInfo` record of the new peer.
This record can be rejected for multiple reasons, summarized [here][peer-handshake].
The received `NodeInfo` record can be invalid (`isNodeInfoInvalid` reason),
the reported node ID may not match the peer ID learned in step (2) (`isAuthFailure` reason),
or it can match the local node ID (`isSelf` reason),
and the local and remote's `NodeInfo` can be incompatible (`isIncompatible` reason).
In any of these cases, 
the method returns an `ErrRejected` error accompanied of the specific reason.

If the connection upgrade succeeds, the method returns the established secret
connection and the `NodeInfo` record of the peer.

## Listen

The `Listen` method produces a TCP listener instance for the provided network address,
and spawns an `acceptPeers` routine to handle the connections [accepted](#accepting-peers)
by the listener.
The `NetAddress` method exports the listen address configured for the transport.

The maximum number of simultaneous incoming connections accepted by the listener
is bound to `MaxNumInboundPeer` plus the configured number of unconditional peers,
using the `MultiplexTransportMaxIncomingConnections` option,
in the node [initialization](https://github.com/tendermint/tendermint/blob/46badfabd9d5491c78283a0ecdeb695e21785508/node/node.go#L559).

This method is called when a node is [started](https://github.com/tendermint/tendermint/blob/46badfabd9d5491c78283a0ecdeb695e21785508/node/node.go#L966).
In case of errors, the `acceptPeers` routine is not started and the error is returned.

## Close

The `Close` method closes the TCP listener created by the `Listen` method,
and sends a signal for interrupting the `acceptPeers` routine.

This method is called when a node is [stopped](https://github.com/tendermint/tendermint/blob/46badfabd9d5491c78283a0ecdeb695e21785508/node/node.go#L1019).

## Cleanup

The method removes the connection established with a peer from the `conns` table.
It is invoked when the connection with a peer is closed.

It also invokes the `Peer` interface method to close the connection associated with a peer.

## AddChannel

Register a channel ID to the transport.

The channel ID is added to the list of supported channel IDs stored in the `NodeInfo` record.
The `NodeInfo` record is exchanged with peers in the Tendermint version handshake.

> This method is not synchronized, so it is assumed that channel IDs are
> registered before the transport is started.
>
> In fact, the only call to this method is performed in the `CustomReactors` constructor
> option of a node, i.e., before the node is started.
> Note that the default list of supported channel IDs, including the default reactors,
> is provided to the transport as its original `NodeInfo` record.

[peer-sts]: https://github.com/tendermint/tendermint/blob/main/spec/p2p/peer.md#authenticated-encryption-handshake
[peer-handshake]:https://github.com/tendermint/tendermint/blob/main/spec/p2p/peer.md#tendermint-version-handshake
[sts-paper]: https://link.springer.com/article/10.1007/BF00124891
[sts-paper-pdf]: https://github.com/tendermint/tendermint/blob/0.1/docs/sts-final.pdf
