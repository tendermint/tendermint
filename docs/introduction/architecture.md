# Tendermint Architectural Overview
_November 2019_

Over the next few weeks, @brapse, @marbar3778 and I (@tessr) are having a series of meetings to go over the architecture of Tendermint Core. These are my notes from these meetings, which will either serve as an artifact for onboarding future engineers; or will provide the basis for such a document.

## Communication

There are three forms of communication (e.g., requests, responses, connections) that can happen in Tendermint Core: *internode communication*, *intranode communication*, and *client communication*. 
- Internode communication: Happens between a node and other peers. This kind of communication happens over TCP or HTTP. More on this below. 
- Intranode communication: Happens within the node itself (i.e., between reactors or other components). These are typically function or method calls, or occasionally happen through an event bus. 
- Client communiation: Happens between a client (like a wallet or a browser) and a node on the network.

### Internode Communication

Internode communication can happen in two ways:
1. TCP connections through the p2p package
    - Most common form of internode communication
    - Connections between nodes are persisted and shared across reactors, facilitated by the switch. (More on the switch below.)
2. RPC over HTTP 
    - Reserved for short-lived, one-off requests
    - Example: reactor-specific state, like height
    - Also possible: websocks connected to channels for notifications (like new transactions)

### P2P Business (the Switch, the PEX, and the Address Book)

When writing a p2p service, there are two primary responsibilities:
1. Routing: Who gets which messages?
2. Peer management: Who can you talk to? What is their state? And how can you do peer discovery? 

The first responsbility is handled by the Switch:
- Responsible for routing connections between peers
- Notably _only handles TCP connections_; RPC/HTTP is separate
- Is a dependency for every reactor; all reactors expose a function `setSwitch`
- Holds onto channels (channels on the TCP connection--NOT Go channels) and uses them to route
- Is a global object, with a global namespace for messages 
- Similar functionality to libp2p

TODO: More information (maybe) on the implementation of the Switch. 

The second responsibility is handled by a combination of the PEX and the Address Book. 

	TODO: What is the PEX and the Address Book? 
	

## node.go 

node.go is the entrypoint for running a node. It sets up reactors, sets up the switch, and registers all the RPC endpoints for a node.

## Types of Nodes
1. Validator Node: 
2. Full Node:
3. Seed Node:

TODO: Flesh out the differences between the types of nodes and how they're configured. 

## Reactors 

Here are some Reactor Facts: 
- Every reactor holds a pointer to the global switch (set through `SetSwitch()`)
- The switch holds a pointer to every reactor (`addReactor()`)
- Every reactor gets set up in node.go (and if you are using custom reactors, this is where you specify that)
- `addReactor` is called by the switch; `addReactor` calls `setSwitch` for that reactor 
- There's an assumption that all the reactors are added before 
- Sometimes reactors talk to each other by fetching references to one another via the switch (which maintains a pointer to each reactor). **Question: Can reactors talk to each other in any other way?**

Furthermore, all reactors expose:
1. A TCP channel
2. A `receive` method 
3. An `addReactor` call

The `receive` method can be called many times by the mconnection. It has the same signature across all reactors.

The `addReactor` call does a for loop over all the channels on the reactor and creates a map of channel IDs->reactors. The switch holds onto this map, and passes it to the _transport_, a thin wrapper around TCP connections.

The following is an exhaustive (?) list of reactors:
- Blockchain Reactor
- Consensus Reactor 
- Evidence Reactor 
- Mempool Reactor
- PEX Reactor

Each of these will be discussed in more detail later.

### Blockchain Reactor 
The blockchain reactor has two responsibilities: 
1. Serve blocks at the request of peers 
2. TODO: learn about the second responsibility of the blockchain reactor 