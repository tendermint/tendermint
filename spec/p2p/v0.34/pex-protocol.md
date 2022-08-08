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
They crawl the network, connecting to random peers, in order to learn as many
peer addresses as possible to provide to other nodes.

## Requesting Addresses

A node requests peer addresses by sending a `PexRequest` message to a peer.

For regular nodes, not operating in seed mode, a PEX request is sent when
the node *needs* peers addresses, a condition checked:

1. When an *outbound* peer is added, causing the node to request peer addresses
   to the new peer
1. Periodically, by the `ensurePeersRoutine`, causing the node to request peer
   addresses to a randomly selected peer

A node needs more peer addresses when its addresses book has less than 1000 records.
It is thus reasonable to assume that the common case is that a peer needs more
peer addresses, so that PEX requests are sent whenever the above two situations happen.

A PEX request is sent when a new *outbound* peer is added.
The same does not happen with new inbound peers
because outbound peers, that the node has dialed, are considered more trustworthy
than inbound peers, that the node has accepted.
Moreover, when a node is short of peer addresses, it dials the configured seed nodes;
since they are added as outbound peers, the node can immediately request peer addresses.

The `ensurePeersRoutine` periodically checks, by default every 30 seconds (`ensurePeersPeriod`),
whether the node has enough outbound peers.
If it does not have, the node tries dialing some peer addresses stored in the Address Book.
As part of this procedure, the node selects a peer at random,
from the set of connected peers retrieved from the switch,
and sends a PEX request to the selected peer.

### Responses

After sending a PEX request is sent to a peer, the node expect to receive,
as a response, a `PexAddrs` message from the peer.
This message encodes a list of peer addresses, which are added to the node's
Address Book, having the peer that sent the PEX response as their source.

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
peer addresses, with target size of `23%` (`getSelectionPercent`) of the
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
An exception is made for the first two PEX requests received from a peer.

> The probably reason is that, when a new peer is added, the two conditions for
> a node to request peer addresses can be triggered with an interval lower than
> the minimum accepted interval.
> Since this is a legit behavior, it should not be punished.

## Seed nodes

A seed node is a node configured to operate in `SeedMode`.

### Crawling peers

Seed nodes crawl the network, connecting to random peers and sending PEX
requests to them, in order to learn as many peer addresses as possible.
More specifically, a node operating in seed mode sends PEX requests in two cases:

1. When a outbound peer is added, and the seed node needs more peer addresses,
   it requests peer addresses to the new peer
1. Periodically, the `crawlPeersRoutine` sends PEX requests to a random set of
   peers, whose addresses are registered in the Address Book

The first case also applies for nodes not operating in seed mode.
The second case replaces the second for regular nodes, as seed nodes do not
run the `ensurePeersRoutine`, as regular nodes,
but run the `crawlPeersRoutine`, which is not run by regular nodes.

The `crawlPeersRoutine` periodically, every 30 seconds (`crawlPeerPeriod`),
starts a new peer discovery round.
First, the seed node retrieves a random selection of peer addresses from its
Address Book.
This selection is produced in the same way as in the random selection of peer
addresses that are [provided](#provided-addresses) to a requesting peer.
Are removed from this selection peers that the seed node has crawled recently,
last than 2 minutes ago (`minTimeBetweenCrawls`).
The remaining peer addresses are registered in the `crawlPeerInfos` table.

The seed node is not necessarily connected to the peer whose address is
selected for each round of crawling.
So, the seed node dials the selected peer addresses.
This is performed in foreground, one peer at a time.
As a result, a round of crawling can take a substantial amount of time.
For each selected peer it succeeds dialing to, this include already connected
peers, the seed node sends a PEX request.

Dialing a selected peer address can fail for multiple reasons.
The seed node might have attempted to dial the peer too many times.
In this case, the peer address is marked as bad in the address book.
The seed node might have attempted to dial the peer recently, without success,
and the exponential `backoffDuration` has not yet passed.
Or the current connection attempt might fail, which is registered in the address book.

Failures to dial to a peer address produce an information that is important for
a seed node.
They indicate that a peer is unreachable, or is not operating correctly, and
therefore its address should not be provided to other nodes.
This occurs when, due to multiple failed connection attempts or authentication
failures, the peer address ends up being removed from the address book.
As a result, the periodically crawling of selected peers not only enables the
discovery of new peers, but also allows the seed node to stop providing
addresses of bad peers.

### Offering addresses

FIXME: do seed nodes run other protocols, in addition to the PEX protocol?

TODO: seed node operation when providing peer addresses

When a PEX request is accepted from an inbound peer, a PEX reply is sent even
it the minimum accepted period between requests is observed.
In addition, once the PEX reply is sent, the peer is disconnected.
The rationale here is that the peer only connected to the seed node in order to
get new peer addresses, and not to maintain a long-term connection.
