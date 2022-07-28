# v0.34 P2P Communicaton layer

## Intro

The goal of this document is to specify the behaviour of the p2p layer in Tenderming v0.34. The existing documents aiming to describe the p2p layer are:

- https://github.com/tendermint/tendermint/tree/master/spec/p2p : Peer information (handshake and addresses); Mconn package;
Tendermint nodes can connect and communicate via p2p to one another. The main high level responsibilities of the p2p layer are  1) establishing and maintaining connections between peers and 2) managing the state of peers. 
- https://github.com/tendermint/tendermint/tree/master/docs/tendermint-core/pex : PEX reactor (*Note* I am not sure that the peer exchange section is valid anymore)

## p2p main concepts
The main components that manage these two broad categories are:
- The Switch
- The PEX reactor 
- Transport
- Different reactors in Tendermint implementing p2p reactor interface

The tables below gives a high level overview of the main functionalities provided by the p2p implementation of Tendermint and the components responsible for each functionality. 

### **Peer communication** 
| Peer discovery | Peer dialing | Accepting connections from peers | Connection management (processing msgs) |
| ---| ---| ---| --- | 
| PEX / config | PEX / Switch | Reactors / Switch | Reactors| 


### **Peer management**
| Peer ranking | Connection upgrading | Evicting| 
| --- | --- | --- |
| PEX / reactors (only marking peers as good/bad) | - | Switch| 


The main states a node can be in are:
- 'Candidate': peers that we have not connected to yet but are discovered. (no explicit list)
- 'Dialing' : peers who are currently being dialed. ( I did not see where this is of particular use. )
- 'Peers' : Connected peers (effectivley a connected state)
- 'Reconnecting': Peers to whom a node is currently reconnecting. The node is trying to establosh a connectiong to these peers and is failing to do so. After a certain amount of time, the peer is simply dropped to be discovered again by the PEX reactor. 
- 'BadPeers' : This is the only list not kept by the switch. It is stored within the address book of the PEX reactor. 

The diagram below shows the lifecycle of an outbound peer:

<img src="img/p2p_state.png" width="50%" title="Outgoing peers lifecycle">Outgoing peers lifecycle</img>


### Peer discovery

Peers are discovered by adding addresses provided in the config file or triggering dials to seed nodes or nodes in the address book. 

When a node is started, it provides the list of persistent peers to the switch by calling `DialPeersAsync`.

Depending on whether the node is a seed node or or not, the PEX reactor constantly runs either `crawlPeersRoutine` or `ensurePeers()` respectively. 
If the node is a seed node, `crawlPeersRoutine` reads peer information randomly from the address book, tries to dial the peer and requests from them information on other peers.

When a node receives information about other peers from a seed note, it sends another request to the node asking for more peers. 

A node also learns of other peers when they try to connect to it. 

### Dialing peers

Every node connects to a set of peers to whom it sends requests via the Switch. 

The PEX reactor ensures a node is connected to peers by running a routine to keep dialing peers in the background until a threshold of connected peers is reached (`MaxNumOutboundPeers`). This way, if a peers' address has been added via an incoming request, it will eventually be dialed. Peers to dial are chosen with some configurable bias for unvetted peers. The bias should be lower when we have fewer peers and can increase as we obtain more, ensuring that our first peers are more trustworthy, but always giving us the chance to discover new good peers. 

As the number of outgoing peers is limited, the reactor will choose the number of addresses to dial taking into account the current number of outgoing connections and ongoing dialings. The addresses to dial will be picked with a bias towards new and vetted peers (TODO define ).

Except for persistent peers, all other peers can be dialed at most `maxAttemptsToDial`. If a node is not connected by that time it is marked as bad. Otherwise, if the dial fails, the node will wait before the next dial. (exponential backoff mechanism). 

If a node needs more peers, the PEX reactor checks first whether peers marked as bad can be reinstated and then also picks a random peer from the store to dial. The PEX reactor relies on the switch to do the actual dialing of a peer.

Once the addresses to dial are known, they are forwarded to the `DialPeersAsync` routine of the switch. Each address is then dialed in parallel and the corresponding peer is added to the `dialing` list. 

**Note** When dialing a peer, the peers' address is not added into the address book as it had to be there in order for the node to dial it in the first place. 


#### *Successful dialing*

When a peer is successfully dialed, it is removed from the `dialing` list and added to the `peers` list. The switch then calls the `InitPeer` and `AddPeer` routines of all the reactors that have registered to it. 

Once a peer is successfully dialed, we ask it to provide us with more peers.

#### *Dialing failed*


### *Removing peers*

Each reactor can call the `StopPeerForError` method of the switch with the ID of the peer that needs to be removed. Then the switch handles stopping the peer (closing the connection to it), and calls the `RemovePeer` method of all reactors registered to the switch.

## Peer ranking

In v0.34 there is no explicit ranking of peers. When choosing peers to dial, there is slight bias towards new and vetted peers. The amount of bias is higher when there are more peers connected to a node. 

In addition to this, peers  can be marked as bad and removed entirely from the potential candidate list. Reactors themselves can also mark peers as bad or good and thus influence the behaviour of the p2p layer. In v0.34 a peer is marked good only from the consensus reactor whenever a peer delivers a correct consensus message (TODO check conditiosn for this). 

A peer is marked as bad in the following cases: 
- It sends too frequent requests for peer information (`PexRequest` messages)
- Returns an error when a node requests peer information from it
- A node is not able to successfully dial a peer after `MaxDialAttempts` and the peer is not persistent (persistent peers are never marked as bad)

The PEX reactor checks periodically whether (TODO what is the exact condition) a peer can be reinstated and removed from the `bad peer` list. However, this is done only if a node does not have sufficient peers. Otehrwise, this list is never revisited. 

## Switch

Every Tendermint reactor regiters itself with the Switch by prividing a Channel ID it is listening to to it. The Switch then forwards messages destined to a particular channel. It is the reactors responisibility to process the messages. 

The switch is implemented as a service and, on start, listens for incoming connections in the background (calling the`acceptRoutine` function). 

Dialing peers, either via the PEX reactor or dialing persistent peers on startup, is done by calling the `DialPeersAsync` routine of the Switch.

When the connection to a peer is established (either by dialing it or accepting an incoming connection from it), the peer address is added to the address book. (The address book is managed by the PEX reactor). 

When a peer is added to the address book, it is marked as connected to and no further new connections are established between the node and this peer. 

The number of peers a node can connect to is set by `MaxNumInboundPeers` and `MaxNumOutboundPeers` respectively. 

## Pex reactor

The PEX reactor is responsible for peer discovery and providing other peers with information about peers a node is already connected to. The PEX reactor receive routine listens to two types of messages: `PexRequest` and `PexAddress`. 

In case of `PexRequest` messages the reactor provides the requesting peer with known peer addresses. The reactor implements request rate limiting by counting the number of requests coming from a single peer. This operation can mark a peer good or bad for a certain amount of time. 

`PexAddress` messages are typically received after a successful request for addresses. Received addresses are added into the address book. Adding to the address book fails if:

- Node is tryign to add self
- The address is private 

A node has a requests map where it stores all requets it issued to a peer asking it for more peer addresses. 


## Notes on diff v0.35+ and v0.34

*Diff 0.36* In v0.36.x there is no difference between the maximum number of inbound and outgoing peers. There is only one parameter to mark the maximum number of connections. But there is a weird tracking of connected peers. Peer is marked as connected by setting it as incoming or outgoing peer. This field has in practice no reference to accept to count incoming and outgoing connections.  

*Diff 0.35+* There is no connection upgrade. 

*Bug/suboptimal 0.36* Pex reactor, line 320. Not really random peer selection.   

*Unclear v0.36* p2p router l781: Why if error == nil do we output this

*Bug v0.36* p2p router l722 - this increment hgappens twice (once for in and once for out )

*v0.36 p2p statesync* If the statesyncing node has only two peers and one of those does not have the requested light block (has not created a snapshot yet for example), statesync will not look for additional peers but will fail to initialize the `StateProvider` and halt. 
