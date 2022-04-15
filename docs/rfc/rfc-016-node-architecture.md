# RFC 016: Node Architecture

## Changelog

- April 8, 2022: Initial draft (@cmwaters)
- April 15, 2022: Incorporation of feedback

## Abstract

The `node` package is the entry point into the Tendermint codebase, used both by the command line and programatically to create the nodes that make up a network. The package has suffered the most from the evolution of the codebase, becoming bloated as developers clipped on their bits of code here and there to get whatever feature they wanted working. 

The decisions made at the node level have the biggest impact to simplifying the protocols within them, unlocking better internal designs and making Tendermint more intuitive to use and easier to understand from the outside. Work, in minor increments, has already begun on this section of the codebase. This document exists to spark forth the necessary discourse in a few related areas that will help the team to converge on the long term makeup of the node.

## Background

> To be written. This will mostly entail the less controversial work that Sam has been doing to clean up the node. 

## Discussion

The following is a list of points of discussion around the architecture of the node:

### Dependency Tree

The node object is currently stuffed with every component that possibly exists within Tendermint. In the constructor, all objects are built and interlaid with one another in some awkward dance. My guiding principle is that the node should only be made up of the components that it wants to have direct control of throughout its life. The node is a service which currently has the purpose of starting other services up in a particular order and stopping them all when commanded to do so. However, there are many services which are not direct dependents i.e. the mempool and evidence services should only be working when the consensus service is running. I propose to form more of a hierarchical structure of dependents which forces us to be clear about the relations that one component has to the other. More concretely, I propose the following dependency tree:

![](./images/node-dependency-tree.svg)

Many of the further discussion topics circle back to this representation of the node.

As pointed out by Michael (@creachadair), It's also important to distinguish two dimensions which may require different characteristics of the architecture. There is the starting and stopping of services and their general lifecycle management. What is the correct oder of operations to starting a node for example. Then there is the question of the needs of the service during actual operation. Does it require access to events? Which data does it need read access from and so forth.

An alternative model and one that perhaps better suits the latter of these dimensions is the notion of an internal message passing system. Either the events bus or p2p layer could serve as a viable transport. This would essentially allow all services to communicate with any other service and could perhaps provide a solution to the coordination problem (presented below) without a centralized coordinator. The other main advantage is that such a system would be more robust to disruptions and changes to the code which may make a hierarchical structure quickly outdated and suboptimal. The addition of message routing is an added complexity to implement, will increase the degree of asynchronicity in the system and may make it harder to debug problems that are across multiple services.

### Coordination of State Advancing Mechanisms

Advancement of state in Tendermint is simply defined in heights: If the node is at height n, how does it get to height n + 1 and so on. Based on this definition we have three components that help a node to advance in height: consensus, statesync and blocksync. The way these components behave currently is very tightly coupled to one another with references passed back and forth. My guiding principal is that each of these should be able to operate completely independent of each other i.e. a node should be able to run solely blocksync indefinitely. There have been several ideas suggested towards improving this flow. I've been leaning strongly towards a centralized system, whereby an orchestrator, in this case the node, decides what services to start and stop. To reach such a state, Tendermint:

-  Should be able to open and close channels dynamically and effectively broadcast which services it is running. In catch up, it should just have the channels it needs open. For example, receiving transactions to a node's mempool while statesyncing isn't a valuable use of bandwidth.
- Should draw a distinction between a service and a process.  
  - The blocksync (and statesync) service, i.e. supplying information for those trying to catch up should only start running once a node has caught up i.e. after running the blocksync and/or state sync *processes*
  - The blocksync and state sync processes have defined termination clauses that inform the orchestrator when they are done and where they finished.
    - One way of achieving this would be that every process both passes and returns the `State` object
  - Consensus doesn't necessarily terminate and thus the line between service and process is more blurry
    - However, one could view the services as running the mempool, evidence and peer state routines which provide information for the consensus engine to run.
    - And just as blocksync uses peer heights to decide when it is finished, Consensus could use peer heights to decide that it has fallen behind and needs to terminate for blocksync to start. 
  - This distinction needs to be communicated in the p2p layer. For example, if a node is state syncing it shouldn't receive requests for snapshots.
- Should know when to switch from one state advancing mechanism to another. The most challenging being to know when to fall back to blocksync.
  - Either the orchestrator instructs consensus to stop if it falls *n* blocks behind consensus (*n* being infinity if, for example, the blocksync service is switched off and we never want consensus to stop)

The orchestrator allows for some deal of variablity in how a node is constructed. Does it just run blocksync, shadowing the head of the chain and be highly available for querying. Does it rely on state sync at all? An important question that arises from this dynamicism is we ideally want to encourage nodes to provide as much of their resources as possible so that their is a healthy amount of providers to consumers. Do we make all services compulsory or allow for them to be disabled? Arguably it's possible that a user forks the codebase and rips out the blocksync code because they want to reduce bandwidth so this is more a question of how easy do we want to make this for users.

### Block Executor

The block executor is an important component that is currently used by both consensus and blocksync to perform the execution grunt work with the application: applying blocks and updating state. Principally, I think it should be the only component that can write (and possibly even read) the block and state stores and we should clean up other references to the storage engine if we can. This would mean:

- The reactors Consensus, BlockSync and StateSync should all import the executor for advancing state ie.  `ApplyBlock` and `BootstrapState`.
- Pruning should also be a concern of the block executor as well as `FinalizeBlock` and `Commit`. This can simplify consensus to focus just on the consensus part.

### The Interprocess communication systems: RPC, P2P, ABCI, and Events

The schematic supplied above shows the relations between the different services, the node, the block executor, and the storage layer. Represented as colourful dots are the components responsible for different roles of interprocess communication. These components permeate throughout the code base, seeping into most services. What can provide powerful functionality on one hand can also become a twisted vine, creating messy corner cases and convoluting the protocols themselves.

The last aspect that hasn't been touched so far is the RPC, metrics and events. With the exception of `/broadcast_tx`, a node could operate normally without any of these components yet would behave as a black box. These components allow for introspection, monitoring and querying of the communal artifact that is the blockchain. Like logging, the other element these have in common is that they are all dispersed throughout every component. On this front we should:

- Look to make sure that the component which controls the information we are monitoring or emitting is the one that controls the publishing of metrics and the pushing of events.
- Understand the information channels differ in audience and usage and repurpose these accordingly. Information source also differs between being global and local. With this in mind we currently have:
  - The RPC providing global data to external clients and applications themselves
  - The RPC also providing local data as well as knobs to control the node for node operators
  - The metrics (and logs) provide local read-only data targeting node operators
  - The events have both global and local data that are almost purely consumed by external clients. 
  - The local events are almost purely used for testing i.e. consesnus
- Have, ideally, such a rubric where we can easily decide in the case of new information whether it makes sense to log it, add it as a metric, expose a new RPC endpoint, or fire a new type of event.

Application developers may have their own ideas around what information should be available and over what form of transport. Following this principal, I think the Node struct should mirror all the API's that the RPC has and that the RPC should be something that can wrap around the node and just extends the API's to intended users via HTTP. I'm aware that we can currently create a "local" RPC client that functionally provides the same purpose, I am just of the opinion that it could be confusing and a non-idomatic way of doing it.

