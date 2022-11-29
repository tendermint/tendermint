*** This is the beginning of an unfinished draft. Don't continue reading! ***

# << Tendermint P2P >>

<!--- > Rough outline of what the component is doing and why. 2-3 paragraphs 
--->

The Tendermint consists of multiple protocols, namely,
- Consensus
- Mempool
- Evidence
- Blocksync
- Statesync

that each plays a role in making sure that validators can produce blocks. These protocols are implemented in so-called reactors (one for each protocol) that encode two functionalities:

- Protocol logic (controlling the local state of the protocols and deciding what messages to send to others, e.g., the rules we find in the arXiv paper)

- Communication. Implement the communication abstractions needed by the protocol on top of the p2p system (e.g., Gossip)
> perhaps we should clarify nomenclature: the Consensus gossip service actually is not implemented by a gossip algorithm but a peer-to-peer system

Tendermint (as many classic BFT algorithms) have an all-to-all communication pattern (e.g., every validator sends a `precommit` to every other full node). Naive implementations, e.g., maintaining a channel between each of the *N* validators is not scaling to the system sizes of typical Cosmos blockchains (e.g., N = 200 validator nodes + seed nodes + sentry nodes + other full nodes). There is the fundamental necessity to restrict the communication. There is another explicit requirement which is called "deployment flexibility", which means that we do not want to impose a completely-connected network (also for safety concerns).

The design decision is to use an overlay network. Instead of having *N* connections, each node only maintains a relatively small number (bounded by constants, say 10 to 50). In principle, this allows to implement more efficient communication (e.g., gossiping), provided that with this small number of connections per node, the system as a whole stays connected. This overlay network 
is established by the **peer-to-peer system (p2p)**, which is composed of the p2p layers of the participating nodes that locally decide with which peers a node keeps connections.

GOAL: what are the consequences of the choices in the config file
 
<!---
# Outline

> Table of content with rough outline for the parts

- [Part I](#part-i---tendermint-blockchain): Introduction of
 relevant terms of the Tendermint
blockchain.

- [Part II](#part-ii---sequential-definition-problem): 
    - [Informal Problem
      statement](#Informal-Problem-statement): For the general
      audience, that is, engineers who want to get an overview over what
      the component is doing from a bird's eye view.
    - [Sequential Problem statement](#Sequential-Problem-statement):
      Provides a mathematical definition of the problem statement in
      its sequential form, that is, ignoring the distributed aspect of
      the implementation of the blockchain.

- [Part III](#part-iii---as-distributed-system): Distributed
  aspects, system assumptions and temporal
  logic specifications.

  - [Incentives](#incentives): how faulty full nodes may benefit from
    misbehaving and how correct full nodes benefit from cooperating.
  
  - [Computational Model](#Computational-Model):
      timing and correctness assumptions.

  - [Distributed Problem Statement](#Distributed-Problem-Statement):
      temporal properties that formalize safety and liveness
      properties in the distributed setting.

- [Part IV](#part-iv---Protocol):
  Specification of the protocols.

     - [Definitions](#Definitions): Describes inputs, outputs,
       variables used by the protocol, auxiliary functions

     - [Protocol](#core-verification): gives an outline of the solution,
       and details of the functions used (with preconditions,
       postconditions, error conditions).

     - [Liveness Scenarios](#liveness-scenarios): when the light
       client makes progress depends heavily on the changes in the
       validator sets of the blockchain. We discuss some typical scenarios.

- [Part V](#part-v---supporting-the-ibc-relayer): Additional
  discussions and analysis


In this document we quite extensively use tags in order to be able to
reference assumptions, invariants, etc. in future communication. In
these tags we frequently use the following short forms:

- TMBC: Tendermint blockchain
- SEQ: for sequential specifications
- LIVE: liveness
- SAFE: safety
- FUNC: function
- INV: invariant
- A: assumption

--->

# Part I - A Tendermint node 

TODO: 
- we should survey here what we think are the expectations of the reactors from p2p? 
    - consensus might need 
        - for liveness that neighborhood is stable
        - proposers are not disconnected?
    - mempool 
        - might just need that a transaction can reach each validator within a reasonable amount of time (this might be achievable if at no point in time the current graph is connected, but, e.g., something along the lines that the union of the graphs over some period in time is connected.)
        - sends ordered list of transactions to peers (ordering might also require stability)
    - evidence: evidence reaches proposer before the evidence expires
    >  talk about the 1-to-1 and the 1-to-many/all delivery guarantees needed by the protocols (do this for each protocol. discuss how these requirements translate into what p2p should guarantee)
    - blocksync and statesync: mainly request/response protocols
- What does p2p expect from the reactors? (don't falsely report bad nodes; this puts requirements on the reactors and perhaps/likely also on the application running on top of ABCI)

Unclassified expectations (need to figure out where they come from)
- establish connections
- end-to-end encrypted
- prioritization of messages
- non blocking
- don't trust peers (DDOS-resistant)
- manual configuration possible

## Context of this document

> mention other components and or specifications that are relevant for this
spec. Possible interactions, possible use cases, etc. 

> should give the reader the understanding in what environment this component
will be used. 

TODO:
- Reactor API
- Network? How do we communicate with other nodes
- Discuss that validators run special set-up, and manage their own neighborhood (hide behind sentry nodes).
   - As a result: the distributed system is composed of
       - (correct) nodes that follow the protocol described here
       - (potentially) adversarial nodes whose behavior deviates to harm the system
       - (correct) nodes that don't follow the protocol to shield themselves but behave in a "nice way"



# Part II - Sequential Definition of the  Problem


##  Informal Problem statement


The p2p layer, specified here, manages the connections of a Tendermint node with other Tendermint nodes. It continuously provides a list of peers ensuring
1. Connectivity. The overlay network induced by the correct nodes in the local neighborhoods (defined by the lists of peers) is sufficiently connected to the remainder of the network so that the reactors can implement communication on top of it that is sufficient for their needs
    > There is the design decision that the same overlay is used by all reactors. It seems that consensus has the strongest requirements regarding connectivity and this defines the required properties
 
    > The overlay network shall be robust agains eclipse attacks. Apparently the current p2p was designed to mixed geographically close and far away neighbors to achieve that.
2. Stability. Typically, connections between correct peers should be stable
    > Even if at every time *t* we satisfy Point 1, if the overlays at times *t* and *t+1* are totally different, it might be hard to implement decent communication on top of it. E.g., Consensus gossip requires a neighbor to know its neighbors *k* state so that it can send the message to *k* that help *k* to advance. If *k* is connected only one second per hour, this is not feasible.
3. Openness. It is always the case that new nodes can be added to the system
    > Assuming 1. and 2. holds, this means, there must always be nodes that are willing to add connections to new peers.
4. Self-healing. The overlay network recovers swiftly from node crashes, partitions, unstable periods, etc. 


## Sequential Problem statement

> should be English and precise. will be accompanied with a TLA spec.

TODO: This seems to be a research question. Perhaps we can find some simple properties by looking at the peer-to-peer systems academic literature from several years ago?

# Part III - Distributed System

> Introduce distributed aspects 

> Timing and correctness assumptions. Possibly with justification that the
assumptions make sense, e.g., it is in the interest of a full node to behave
correctly 

> should have clear formalization in temporal logic.

## Incentives

TODO: 
- who will follow the protocol who won't
- validators hiding behind sentries (they have an incentive to not run it)
- what can be incentives/strategies of bad nodes 
     - DNS
     - filling up all your connections and then disconnecting you
     - feeding your reactors with garbage
     - corrupt overlay to harm protocols running on top, e.g., isolating validators to prevent them from being proposers, but using them to vote for proposals from the bad nodes

general question (is it likely? do we care)

## Computational Model

TODO: 
- partially synchronous systems?
- nodes maintain  long-term persistent identity (public key)
- nodes interact by exchanging messages via encrypted point-to-point communication channels (connections?)
- deployment flexibility: deployment among multiple administrative domains; administrators may decide whether to expose nodes to the public network; not completely connected

## Distributed Problem Statement

TODO
- peer discovery
    - seed nodes
    - persistent peers (provided by operator; configuration?)
    - peer exchange protocol
- address book
- establishing and managing connections

TODO: notation
- connection vs. channel

### Design choices

> input/output variables used to define the temporal properties. Most likely they come from an ADR

The p2p layer is
    - running the peer exchange protocol PEX (in a reactor)
    - using input from the operator (addresses)
    - responding to other peers wishing to connect
> the latter might just be the result of the first two points on the other peer


TODO: The following two points seem to be implementation details/legacy design decisions
- communicate to the reactors over the reactor API
- I/O
   - dispatch messages incoming from the network to the reactors
   - send messages incoming from the reactors to the network (the peers the messages should go to) 
- number of connections is bounded by constants, say 10 to 50

### Temporal Properties

> safety specifications / invariants in English 

TODO: In a good period, *p* should stay connected with *q*.

> liveness specifications in English. Possibly with timing/fairness requirements:
e.g., if the component is connected to a correct full node and communication is
reliable and timely, then something good happens eventually. 

should have clear formalization in temporal logic.


### Solving the sequential specification

> How is the problem statement linked to the "Sequential Problem statement". 
Simulation, implementation, etc. relations 


# Part IV - Protocol

> Overview


## Definitions

### Data Types

### Inputs


### Configuration Parameters

### Variables

### Assumptions

### Invariants

### Used Remote Functions / Exchanged Messages

## <<Core Protocol>>

### Outline

> Describe solution (in English), decomposition into functions, where communication to other components happens.


### Details of the Functions

> Function signatures followed by pseudocode (optional) and a list of features (required):
> - Implementation remarks (optional)
>   - e.g. (local/remote) function called in the body of this function
> - Expected precondition
> - Expected postcondition
> - Error condition


### Solving the distributed specification

> Proof sketches of why we believe the solution satisfies the problem statement.
Possibly giving inductive invariants that can be used to prove the specifications
of the problem statement 

> In case the specification describes an existing protocol with known issues,
e.g., liveness bugs, etc. "Correctness Arguments" should be replace by
a section called "Analysis"



## Liveness Scenarios



# Part V - Additional Discussions





# References

[[block]] Specification of the block data structure. 

[[RPC]] RPC client for Tendermint

[[fork-detector]] The specification of the light client fork detector.

[[fullnode]] Specification of the full node API

[[ibc-rs]] Rust implementation of IBC modules and relayer.

[[lightclient]] The light client ADR [77d2651 on Dec 27, 2019].

[RPC]: https://docs.tendermint.com/master/rpc/

[block]: https://github.com/tendermint/spec/blob/master/spec/blockchain/blockchain.md

[TMBC-HEADER-link]: #tmbc-header.1
[TMBC-SEQ-link]: #tmbc-seq.1
[TMBC-CorrFull-link]: #tmbc-corr-full.1
[TMBC-Auth-Byz-link]: #tmbc-auth-byz.1
[TMBC-TIME_PARAMS-link]: tmbc-time-params.1
[TMBC-FM-2THIRDS-link]: #tmbc-fm-2thirds.1
[TMBC-VAL-CONTAINS-CORR-link]: tmbc-val-contains-corr.1
[TMBC-VAL-COMMIT-link]: #tmbc-val-commit.1
[TMBC-SOUND-DISTR-POSS-COMMIT-link]: #tmbc-sound-distr-poss-commit.1

[lightclient]: https://github.com/interchainio/tendermint-rs/blob/e2cb9aca0b95430fca2eac154edddc9588038982/docs/architecture/adr-002-lite-client.md
[fork-detector]: https://github.com/informalsystems/tendermint-rs/blob/master/docs/spec/lightclient/detection.md
[fullnode]: https://github.com/tendermint/spec/blob/master/spec/blockchain/fullnode.md

[ibc-rs]:https://github.com/informalsystems/ibc-rs

[FN-LuckyCase-link]: https://github.com/tendermint/spec/blob/master/spec/blockchain/fullnode.md#fn-luckycase

[blockchain-validator-set]: https://github.com/tendermint/spec/blob/master/spec/blockchain/blockchain.md#data-structures
[fullnode-data-structures]: https://github.com/tendermint/spec/blob/master/spec/blockchain/fullnode.md#data-structures

[FN-ManifestFaulty-link]: https://github.com/tendermint/spec/blob/master/spec/blockchain/fullnode.md#fn-manifestfaulty

[arXiv]: https://arxiv.org/abs/1807.04938