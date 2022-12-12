# Consensus Reactor

Tendermint Core is a state machine replication framework and part of the stack used in the Cosmos ecosystem to build distributed applications.
Tendermint Core uses its northbound API, ABCI, to communicate with applications.
South of Tendermint Core, is the OS' network stack.

Tendermint Core is implemented in a modular way, separating protocol implementations in **reactors**.
Reactors communicate in with their counterparts on other nodes using the P2P layer, through what we will call the **P2P-I**.


```
                                                      SDK Apps
                                                   ==================
 Applications                                        Cosmos SDK    
====================================ABCI=============================
  Mempool Reactor     Evidence Reactor    Consensus Reactor   PEX ...
- - - - - - - - - - - - - - - P2P-I - - - - - - - - - - - - - - - - - 
                                 P2P
=====================================================================
                            Network Stack
```

This document focuses on the interactions between the Consensus Reactor and the P2P layer.
The Consensus reactor is, itself, divided into two layers, State and Communication. 

The **State** layer keeps the state and transition functions described in the [The latest gossip on BFT consensus](https://arxiv.org/abs/1807.0493).

The **Communication** layer handles messages received through the P2P layer, triggering updates of peer information and, when conditions are met, of the consensus state.
The communication layer also reacts to events in the consensus state and, periodically, propagates information on the node's state to its neighbors. 
It is in this layer that partial synchrony assumptions are combined with P2P primitives to provide Tendermint BFT's liveness guarantees.
Exchanges between State and Communication use multiple forms, but we will call them all **Gossip-I** here.


```
...
==========ABCI=========

  |     State       |
  |.....Gossip-I....| Consensus Reactor
  |  Communication  |

- - - - - P2P-I - - - -
         P2P
=======================
    Network Stack
```

The goal here is to understand what the State layer requires from the Communication layer (Gossip-I) and what the Communication layer requires from P2P (P2P-I) in order to satisfy the State layer needs.

# Status

This is a Work In Progress. It has not been reviewed and is far from completion. 

Some descriptions only resemble TLA+. They will be rigorously rewritten later.



# Outline
> **TODO**



# Part 1: Background

Here we assume that you understand the Tendermint BFT algorithm, which has been described in multiple places, such as [here](../).

The Tendermint BFT algorithm assumes that a **Global Stabilization Time (GST)** exists, after which communication is reliable and timely:

> **Note**  
> Eventual $\Delta$-Timely Communication: There is a bound $\Delta$ and an instant GST (Global Stabilization Time) such that if a correct process $p$ sends message $m$ at time $t \geq \text{GST}$ to a correct process $q$, then $q$ will receive $m$ before $t + \Delta$.

Tendermint BFT also assumes that that it is used to provide "Gossip Communication":

> **Note**    
> Gossip communication: (i)If a correct process $p$ sends some message $m$ at time $t$, all correct processes will receive $m$ before $\text{max} \{t,\text{GST}\} + \Delta$.
Furthermore, (ii)if a correct process $p$ receives some message $m$ at time $t$, all correct processes will receive $m$ before $\text{max}\{t,\text{GST}\} + \Delta$.

Because Gossip Communication requires even messages sent before GST to be reliably delivered between correct processes ($t$ may be smaller than GST in (i)) and because GST could take arbitrarily long to arrive, in practice, implementing this property would require an unbounded message buffer.

However, while Gossip Communication is a sufficient condition for Tendermint BFT to terminate, it is not strictly necessary that all messages from correct processes reach all other correct processes.
What is required is for either the message to be delivered or that, eventually, some newer message, with information that supersedes that in the first message, to be timely delivered.
In Tendermint BFT, this property is seen, for example, when nodes ignore proposal messages from prior rounds.

It seems reasonable, therefore, to formalize the requirements of Tendermint in terms of a weaker, *best-effort* communication guarantee, and to combine it with GST later, to ensure eventual termination.

> **Note**   
> Best Effort communication: continuously forward any non-superseded messages to connected nodes.   
> Best effort Gossip +  $\Diamond\Delta$-Timely Communication ~> $\Diamond$ Termination.

> **Warning**    
> If gossip is **best-effort**, so should be P2P.

It is also assumed that, after GST, timeouts do not expire precociously.

### Supersession
We say that a message $lhs$ supersedes a message $rhs$ if after receiving $lhs$ a process would make at least as much progress as it would by receiving $rhs$ and we note is as follows: $\text{lhs} \overset{s}{>} \text{rhs}$

> **TODO**    
> Better define this





## The Consensus State Layer - State

### Provides o Applications and other Reactors

This part is covered by the ABCI specification.


### Provides to the Communication Layer

A message supersession operator:

`SuperS(lhs,rhs)` is true if and only if $\text{lhs} \overset{s}{>} \text{rhs}$


### Requires from the Communication Layer

Messages received by a node change its State or are discarded. 
If they they change the State, even if by just getting added to a set of received messages, they the original message need not be forwarded. Instead, a new message, with the same contents, is generated from the State of the node. Why does this matter? Because it let's us think in terms of messages being sent to neighbors only, not forwarded on the overlay.

> **[REQ-STATE-GOSSIP-NEIGHBOR_CAST]**   
> Best Effort Neighbor-Cast: continuously resend any non-superseded messages to neighbors (connected nodes).

It should be provable that    
[REQ-STATE-GOSSIP-NEIGHBOR_CAST] +  $\Diamond\Delta$-Timely Communication ~> $\Diamond$ Termination.



## Communication, AKA Gossip


```tla+
SuperS(lhs,rhs): the supersession operator  
Ne[p]: neighbor set of p
Msgs[p]: the messages for which GossipCast has been invoked at p
DMsgs[p]: set of messages GossipDelivered by p
```

### Provides to State (Gossip-I)

[GOSSIP-CAST-NEIGHBORS]

To gossip-cast, add message to the outbound messages set.

```tla+
GossipCast(p,m): Msgs[p]' = Msgs[p] \cup {m} 
```

All outbound messages will be delivered to neighbors at the time the message was sent, or the neighbor disconnects, or a superseding messages makes sending the message useless.

```tla+
GossipCastDelivery(p,q,m) == 
    q \in Ne[p] /\ m \in Msgs[p] ~> \/ m \in DMsgs[q]
                                    \/ m \notin Msgs[p] /\ \E mn \in Msgs[p]: SuperS(mn,m)
                                    \/ q \notin Ne[p]
```




[GOSSIP-CAST-FORWARD]

For every message received, either the message itself is forwarded or a superseding message is broadcast.

```tla+
m \in DMsgs[p] ~> \/ m \in Msgs[p] 
                  \/ \E mn \in Msgs[p]: SuperS(mn,m)
```


#### Current Implementations

[GOSSIP-CAST-NEIGHBORS]

- Go-routines
    - Looping go-routines continuously check the Consensus state for conditions that trigger message sending.
    - For each set of conditions met, messages are created and sent using the P2P-I (Send and TrySend).
    - If the conditions didn't change from the previous iteration, the message created is the same as before, effectively implementing a message retransmission.
    - If the conditions have changed, a new message is created, effectively superseding the message sent on the previous iteration.
    - Superseded messages will never be sent through P2P-I again and therefore need not be maintained. (TODO: Argue that this is still valid refinement mapping).
    - From the point of view of the Gossip layer, Send and TrySend are similar.

- Pub-Sub
    - Upon certain changes on the Consensus state (detected through internal pub-sub) broadcast messages
        - These messages are not discarded, even if superseding messages are created, unless there is a reconnection.
            - Buffer bounding here comes from superseding messages only being generated if the algorithm made some progress, which implies (maybe) some communication with neighbors and therefore that the message was actually sent on the TCP channel?Does not seem really true, since progress means messages coming in, but not necessarily going out. 
            But if messages are not going out, a disconnection will happen and the buffer will be emptied.
            - On node reconnection, only the latest (superseding) messages are retransmitted.

Considering the two forms of communication above, Msgs is bounded.



[GOSSIP-CAST-FORWARD]

- `Receive` function - loop: receive and validate message; invoke PeerState methods to handle contents, updating the state of nodes, which generate superseding messages.

> **Warning**   
> Are there purely forwarded messages?


### Requires from P2P (P2P-I)

[CONCURRENT-CONNECTION]

Ability to connect to multiple nodes concurrently

> P2P provides 1-to-1 interface, while gossip must provide 1-to-many.

[CHURN-DETECTION]

Connection and Disconnection alert

> Gossip needs to keep track of neighbors

[UNAMBIGUOUS-SOURCE-IDENTIFICATION]

Ability to discern sources of messages received

> ~~Needed to identify duplications.~~   
    - Duplication is handled by having messages in the consensus layer be idempotent   
> Needed to address sender ins request response/situation


[NON-REFUTABILITY]

Non-refutability of messages - Is this a gossip or a consensus requirement?



[UNICAST]
- Ability address messages to a single neighbor

#### Current implementations

[CONCURRENT-CONNECTION]

- Inherited from the network stack
- Driven by PEX and config parameters

[CHURN-DETECTION]
- `AddPeer`
- `RemovePeer`

[UNAMBIGUOUS-SOURCE-IDENTIFICATION]

- Node cryptographic IDs

[NON-REFUTABILITY]

- Authentication/Signing

[UNICAST]

- `Send(Envelope)`/`TrySend(Envelope)`
    - Enqueue and forget. 
    - Disconnection and reconnection may cause drops. 
    - Enqueuing may block for a while in `Send`, but not on `TrySend`



#### Non-requirements
- Non-duplication
    - Gossip itself will duplicate messages
    - Idempotency must be implemented by the application

- Ability to send message to multiple neighbors
    - Gossip itself can implement this, given other abilities



## References
- [The latest gossip on BFT consensus](https://arxiv.org/abs/1807.0493)











## TODO/Questions/Thoughts
- Flow of information
    - Option a - message set based (works everywhere, but harder to implement)
        - The State layer updates its state
        - The gossip layer adds messages to sets
        - Messages in the sets are added to neighbor nodes' sets
        - Messages in the sets are delivered to consensus
        - Messages delivered and present in the neighbor sets are forgotten
            - Duplication is detected at the gossip layer
        - State/Gossip talk through a clear API
    - Option b - state propagation based (works only if all nodes do all updates (validators), but easier to implement)
        - The consensus layer updates its state
        - The gossip layer compares local state and remote state estimate and propagates possibly relevant state updates
            - Proposal Seen
            - Prevoted
            - Precommitted
        - Messages don't need to be forwarded as loop on the neighbors will propagate their own state
        - Messages are forgotten as soon as sent
        - Messages are forgotten as soon as received
        - Gossip reaches into the State guts to capture the state
            - API to retrieve the state and reduce coupling?
    - Solution (current?)
        - mix state and message propagation?
        - non-validators propagate messages
            - State may include a set of messages to be propagated, turning b into a
        - validators propagate state
            - if all state is represented as set of messages, turns a into b
        - if full nodes update all states, then use b instead
        - What state is captured today?
        - What messages are sent today?

- Where should we "consume" GST?
    - If in Gossip, then P2P is simpler and provides a best effort multicast/deliver and is up to Gossip to combine that with GST to provide eventual info delivery.
    - If in P2P, then P2P is more complicated, provides eventual delivery, and Gossip is simpler and just uses it.
    - Where is it (attempted) used in the current implementation?




# Scratch

### Provides to State

- BroadcastMessage(m)
    - Adds m to a set `S` of messages to be broadcast.
        > Current implementation uses a queue/queue to store messages, but this is a best effort in ordering the spread of messages since asynchronism and the existence of multiple paths in the network can invert the order. Hence, globally, the collections may be seen as sets.  

        > Current implementation has a maximum size for the set and if more messages are enqueued to enter the set using go-routines.

    - `\A m \in `: Send m to all neighbors.
       > Neighbors known to have seen the message may be filtered.
   - Any messages received are are added to the local set `S`.
   
- DeliverMessage(m)
    - `\A m \in S`, DeliveredMessage(m) is invoked in the State layer.
    - DeliveredMessage(m) is invoked exactly once.


### Requires from P2P

If GST is not an assumption of P2P, then it will provide

- [Best Effort Info Delivery] `\A m1`, Send(m1) invoked implies that transmission of m1 will be attempted until Receive(m1) is invoked or `\E m2`, m2 supersedes, and Send(m2) is invoked.
    > P2P cannot guarantee eventual delivery of messages since it would require potential infinite memory to store messages, while waiting for GST, to deliver to correct nodes in other partitions.



GST ~> Eventual Connectivity

P2P + Eventual Connectivity ~> [Eventual Info Delivery]

- [Eventual Info Delivery] `\A m1`, Send(m1) invoked implies Receive(m1) is invoked or `\E m2`, m2 supersedes, and Receive(m2) is invoked.

    > Current implementation does not consider superseding, except if a decision is reached, in which case messages may not be dropped on the current node, but if the message arrives at the next node and it has decided, then the message is dropped, so it seems like it could be dropped locally as well without loss of correctness.

    > Current implementation may lose messages in `S` (if peers are removed and re-added (messages are dropped with the connection and not re-send once the connection is reestablished))








- P2P is Best effort (connectedness/delivery)
- GST is assumed
    - Clarify GST. 
        - Is it between all correct nodes? A certain number of correctness nodes?
        - Eventually all connections are stable? Always eventually there will be a connection?
- P2P + GST ~> Eventual connectivity/delivery
- Eventual Connectivity ~> “Gossip” communication

