# P2P Communicaton layer


Tendermint nodes can connect and communicate via p2p to one another. 

The main components of the p2p layer in v0.34 are:

- Peers
- Switch
- Transport
- The PEX reactor for peer discovery
- Reactor specific `Receive` routines

Every node connects to a set of peers to whom it sends requests via the Switch.  A node can receive requests via a `Receive` routine which is implemented by each of the existing reactors. 

## Peer ranking

Peers are marked as bad if a connection to them fails, they send us request too frequently or the peer errors. Marking a peer as bad is done atomically by putting the peer in a list of bad peers and removing it from the address book. Periodically the peer reactor is checking whether bad peers can be reinstated. 

Persistent peers are not marked as bad (TODO verify this again).

## Switch

- Every Tendermint reactor regiters itself with the Switch by prividing a Channel ID it is listening to to it. The Switch then forwards messages destined to a particular channel. It is the reactors responisibility to process the messages. 

The switch is implemented as a service and, on start, listens for incoming connections in the background (calling the`acceptRoutine` function). 

On startup each node starts dialing peers it knows about by calling the `DialPeersAsync` routine of the Switch.

When the connection to a peer is established (either by dialing it or accepting an incoming connection from it), the peer address is added to the address book. (The address book is managed by the PEX reactor). 

When a peer is added to the address book, it is marked as connected to and no further new connections are established between the node and this peer. 

The number of peers a node can connect to is set by `MaxNumInboundPeers` and `MaxNumOutboundPeers` respectively. 

*Diff 0.36* In v0.36.x there is no difference between the maximum number of inbound and outgoing peers. There is only one parameter to mark the maximum number of connections. But there is a weird tracking of connected peers. Peer is marked as connected by setting it as incoming or outgoing peer. This field has in practice no reference to accept to count incoming and outgoing connections.  

*Diff 0.35+* There is no connection upgrade. 

*Bug/suboptimal 0.36* Pex reactor, line 320. Not really random peer selection.  

## Node states

- 'Candidate': peers that we have not connected to yet but are discovered. (no explicit list)
- 'Dialing' : peers who are currently being dialed. ( I did not see where this is of particular use. )
- 'Peers' : Connected peers (effectivley a connected state)
- 'Reconnecting': Peers to whom a node is currently reconnecting. The node is trying to establosh a connectiong to these peers and is failing to do so. After a certain amount of time, the peer is simply dropped to be discovered again by the PEX reactor. 
- 'BadPeers' : This is the only list not kept by the switch. It is stored within the address book of the PEX reactor. 

## Pex reactor

The PEX reactor is responsible for peer discovery. The PEX reactor receive routine listens to two types of messages: `PexRequest` and `PexAddress`. 

In case of `PexRequest` messages the reactor provides the requesting peer with known peer addresses. The reactor implements request rate limiting by counting the number of requests coming from a single peer. This operation can mark a peer good or bad for a certain amount of time. 

`PexAddress` messages are typically received after a successful request for addresses. Received addresses are added into the address book. Adding to the address book fails if:

- Node is tryign to add self

The PEX reactor ensures a node is connected to peers by running a routine to keep dialing peers in the background until a threshold of connected peers is reached. This way, if a peers' address has been added via an incoming request, it will eventually be dialed. 

If a node needs more peers, the PEX reactor checks first whether peers marked as bad can be reinstated and then also picks a random peer from the store to dial. The PEX reactor relies on the switch to do the actual dialing of a peer. 