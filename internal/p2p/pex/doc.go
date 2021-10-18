/*
Package PEX (Peer exchange) handles all the logic necessary for nodes to share
information about their peers to other nodes. Specifically, this is the exchange
of addresses that a peer can use to discover more peers within the network.

The PEX reactor is a continuous service which periodically requests addresses
and serves addresses to other peers. There are two versions of this service
aligning with the two p2p frameworks that Tendermint currently supports.

The reactor is embedded with the new p2p stack and uses the peer manager to advertise
peers as well as add new peers to the peer store. The V2 reactor passes a
different set of proto messages which include a list of
[urls](https://golang.org/pkg/net/url/#URL).These can be used to save a set of
endpoints that each peer uses. The V2 reactor has backwards compatibility with
V1. It can also handle V1 messages.

The reactor is able to tweak the intensity of it's search by decreasing or
increasing the interval between each request. It tracks connected peers via a
linked list, sending a request to the node at the front of the list and adding
it to the back of the list once a response is received. Using this method, a
node is able to spread out the load of requesting peers across all the peers it
is currently connected with.

With each inbound set of addresses, the reactor monitors the amount of new
addresses to already seen addresses and uses the information to dynamically
build a picture of the size of the network in order to ascertain how often the
node needs to search for new peers.
*/
package pex
