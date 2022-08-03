# Peer Exchange Protocol

The Peer Exchange (PEX) protocol enables nodes to exchange peer addresses, thus
implementing a peer discovery mechanism.

The PEX protocol uses two messages:

- `PexRequest`: sent by a node to [request](#requesting-addresses) peer
  addresses to a peer
- `PexAddrs`: a list of peer addresses [provided](#providing-addresses) to a
  peer as response to a `PexRequest` message

While all nodes, with few exceptions, participate on the PEX protocol,
a subset of nodes, configured as [seed nodes](#seed-nodes) have a particular
role in the protocol.
They crawl the network, connecting to random peers, in order to collect as many
peer addresses as possible to offer to other nodes.

## Requesting Addresses

The node requests peer addresses by sending a `PexRequest` message to a peer.

For regular nodes, a PEX request is sent in the following two situations,
provided that the node *needs* peer addresses:

1. When a outbound peer is added, the node requests peer addresses to the new peer
1. The `ensurePeersRoutine` periodically requests peer addresses to a randomly
   selected peer to which the node is connected

A node needs more peer addresses when its addresses book has less than 1000 records.
So, it is reasonable to assume that, most of the time, a node needs more peer addresses,
and PEX requests are sent whenever the above two situations happen.

A PEX request is sent only to new outbound peers for two reasons.
First, because peers that the node has dialed are considered more trustworthy
than peers that were accepted because they dialed the node.
Second, when a node is short of peer addresses (e.g., at initialization),
it dials the configured seed nodes.
Once connected to a seed node, the node immediately requests peer addresses.

The `ensurePeersRoutine` instructs the switch to dial peers when the node does
not have enough outbound peers.
This condition is verified at regular intervals, every 30 seconds by default
(`ensurePeersPeriod`).
While this condition is not part of the Peer Exchange protocol,
whenever it is observed, the node sends a PEX request to a random peer,
from the peers that the switch reports as connected peers.

When a node receives a `PexAddrs` message from a peer,
it adds all the peer addresses encoded in the message to the Address Book.
The peer that sent the message is set as the source of the added peer addresses.

> TODO: what happens to added peers? if the peers are already in the Address Book?
>
> This should be described in the Address Book implementation, as it has
> nothing to do with the PEX protocol.

### Misbehavior

Sending multiple PEX requests to a peer, before receiving a reply from it,
is considered a misbehavior.
To prevent it, the node maintains a `requestsSent` set of outstanding
requests, indexed by destination peers.
While a peer ID is present in the `requestsSent` set, the node does not send
further PEX requests to that peer.
A peer ID is removed from the `requestsSent` set when a PEX response is
received from it.

Sending a PEX response to a peer that has not requested peer addresses
is also considered a misbehavior.
So, if a PEX response is received from a peer that is not registered in
the `requestsSent` set, a `ErrUnsolicitedList` error is produced.
This leads the peer to be disconnected and marked as a bad peer.

## Providing Addresses

When a node receives a `PexRequest` message from a peer,
it replies with a `PexAddrs` message containing a list of peer addresses.

The list of peer addresses provided to a peer is essentially a random selection
of entries stored in the node's Address Book.
It is composed by from `32` (`minGetSelection`) to `250` (`maxGetSelection`)
peer addresses, with target length of `23%` (`getSelectionPercent`) of the
number of entries in the address book.
Of course, the length of the list cannot be greater than the size of the address book.

> FIXME: how really random the selected list of peer addresses is?

### Misbehavior

Requesting peer addresses to often is considered a misbehavior.
Since node are expected to send PEX requests every `ensurePeersPeriod`,
the minimum accepted interval between requests from the same peer is set
to `ensurePeersPeriod / 3`, 10 seconds by default.

The `receiveRequest` method is responsible for verifying this condition.
The node keeps a `lastReceivedRequests` map with the time of the last PEX
request received from every peer.
If the interval between successive requests is less than the minimum accepted
one, the peer is disconnected and marked as a bad peer.
An exception is made for the first pair of PEX requests received from a peer.

> The probably reason is that, when a new peer is added, the two conditions for
> a node request peer addresses can be triggered with an interval lower than
> the minimum accepted interval.
> Since this is a legit behavior, it should not be punished.

## Seed nodes

The operation of nodes configured as seed nodes is different.

A seed node sends PEX requests in the following two situations:

1. When a outbound peer is added, and the node needs more peer addresses, the
   node requests peer addresses to the new peer
1. The `crawlPeersRoutine` periodically requests peer addresses to a set of
   peers randomly selected from the Address Book

The first situation is the same as for regular nodes.
The second situation replaces the second situation for regular nodes, as a seed
node does not run the `ensurePeersRoutine`, but runs the `crawlPeersRoutine`
for discovering peer addresses that is not run by regular nodes.

The `crawlPeersRoutine` periodically sends a round of PEX requests,
every 30 seconds (`crawlPeerPeriod`).
Requests are expected to be sent to an essentially random selection of peers
present in the seed node's Address Book.
This list is produced in the same way as in the random selection of peer
addresses that are [provided](#provided-addresses) to a requesting peer.
The seed node does not send PEX requests to all selected peers.
In particular, it does not send requests to peers crawled recently, last than 2
minutes ago (`minTimeBetweenCrawls`).

The seed node is not necessarily connected to the peers selected at each round
of crawling.
So, the seed node dials the selected peers, which is performed in foreground,
one peer at a time.
For each peer it succeeds dialing to, this include already connected peers,
the seed node sends a PEX request.
The failed attempts to connect to selected peers are, in they turn, registered
by the seed node in its Address book.
Peers with multiple failed connection attempts during a week are removed from
the Address Book.
As a result, the periodically crawling of selected peers not only enables the
discovery of new peers, but also allows the seed node to stop providing
addresses of peers that are probably offline.

TODO: seed node operation when providing peer addresses

When a PEX request is accepted from an inbound peer, a PEX reply is sent even
it the minimum accepted period between requests is observed.
In addition, once the PEX reply is sent, the peer is disconnected.
The rationale here is that the peer only connected to the seed node in order to
get new peer addresses, and not to maintain a long-term connection.
