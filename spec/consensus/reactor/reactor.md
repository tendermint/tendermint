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

The **Communication** layer (AKA, Gossip), keeps state and transition functions related to the gossiping with other nodes.
This layer reacts to messages received through the P2P layer by updating the layer internal state and, when conditions are met, calling into the State.
It also handles the broadcasts made by State.
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

> **Eventual $\Delta$-Timely Communication**: There is a bound $\Delta$ and an instant GST (Global Stabilization Time) such that if a correct process $p$ sends message $m$ at time $t \geq \text{GST}$ to a correct process $q$, then $q$ will receive $m$ before $t + \Delta$.

Tendermint BFT also assumes that this property is used to provide **Gossip Communication**:

> **Gossip communication**: (i)If a correct process $p$ sends some message $m$ at time $t$, all correct processes will receive $m$ before $\text{max} \{t,\text{GST}\} + \Delta$.
Furthermore, (ii)if a correct process $p$ receives some message $m$ at time $t$, all correct processes will receive $m$ before $\text{max}\{t,\text{GST}\} + \Delta$.

Because Gossip Communication requires even messages sent before GST to be reliably delivered between correct processes ($t$ may happen before GST in (i)) and because GST could take arbitrarily long to arrive, in practice, implementing this property would require an unbounded message buffer.

However, while Gossip Communication is a sufficient condition for Tendermint BFT to terminate, it is not strictly necessary that all messages from correct processes reach all other correct processes.
What is required is for either the message to be delivered or that, eventually, some newer message, with information that supersedes that in the first message, to be timely delivered.
In Tendermint BFT, this property is seen, for example, when nodes ignore proposal messages from prior rounds.

> **Supersession**: We say that a message $lhs$ supersedes a message $rhs$ if after receiving $lhs$ a process would make at least as much progress as it would by receiving $rhs$ and we note is as follows: $\text{lhs}.\text{SSS}(\text{rhs})$.   
> **TODO**: Better definition.

It seems reasonable, therefore, to formalize the requirements of Tendermint in terms of a weaker, *best-effort* communication guarantees and to combine them with GST outside of the communication layer, to ensure eventual termination.

> **Note**   
> It is also assumed that, after GST, timeouts do not expire precociously.





# Part 2: Specifications
## The Consensus State Layer - State
The consensus layer is where the actions of the Tendermint BFT are implemented.
Actions are executed once certain pre-conditions apply, such as timeout expirations or reception of information from particular subsets of the nodes in the system., neighbors or not.

An action may require communicating with applications and other reactors, for example to gather data to compose a proposal or to deliver decisions, with the P2P layer to communicate with other nodes.

### Northbound Interaction
Communication of the Consensus Reactor with applications is covered by the [ABCI](../../abci/) specification.

Communication with the Mempool to build an initial block proposal happens through function calls.
This is an "optional" step, though, as the actual proposal is made by the application through the ABCI method `prepareProposal`.
Hence we ignore other reactors here and refer to the northbound interaction as being only to Applications.

#### Requires from Applications
* Reasonable parameters to drive the execution of Consensus
    * a validator sets with less than 1/3 of voting power belonging to byzantine nodes
* Timely creation of proposals
* Timely processing of decisions
* ...

See [ABCI](../../abci/abci%2B%2B_app_requirements.md)

#### Provides to Applications
* Fairness proposing values from all validators
* Eventual delivering decisions, as long as assumptions are met
* ...

See [ABCI](../../abci/abci%2B%2B_tmint_expected_behavior.md)


### Southbound Interaction
The State layer interacts southbound only with the Communication layer.

#### Vocabulary

```scala
type Proc = Int
const Step = Map("Proposal" -> 1,  "Prevote" -> 2, "Precommit" -> 3)
type HRS = {height: Int, round: Int, step: Step.Domain}
type StateMessage = {
        hrs: HRS, 
        payload: str,                                   //TODO: Define payloads?
        src: Proc
}
                                                        //TODO: Where to include the Communication messages?

Ne: Proc -> Proc                                        //Neighbor sets
BMsgs: Proc -> Set[StateMessage]                        //Messages broadcast by State
DMsgs: Proc -> Set[StateMessage]                        //Messages delivered to State
Msgs[p]: Proc -> Set[StateMessage]                      //Messages to be sent/forwarded (subset BMsgs[p] U DMsgs[p])

SSS(lhs,rhs): (StateMessage, StateMessage) => bool      //The supersession operator 
```

> **TODO**   
> Where to best place this? As a single vocabulary for all layers or as separate vocabularies for each interface?
> Consider that properties may be required for multiple vocabularies.

### Requires from the Communication Layer
The State layer uses the Communication layer to broadcast information to all nodes in the system and, therefore in terms of API, a single method to broadcast messages is required, `broadcast(msg)`. 

Although ideally the State layer shouldn't be concerned about the implementation details of `broadcast`, it would be unreasonable to do so as it could impose unattainable behavior to the Communication layer.
Specifically, because the network is potentially not complete, message forwarding may be required to implement `broadcast` and, as discussed previously, this would require potentially infinite message buffers on the "relayers".
Hence, the State layer can only require a **best-effort** in broadcasting messages, allowing the communication layer to quit forwarding messages that are no longer useful or, in other words, which have been **superseded**.

> **Note**    
> Since it would be impossible for all the nodes to know immediately when a message was superseded, we use non-superseded as a synonym to "not known by the node to have been superseded".

**[REQ-STATE-GOSSIP-NEIGHBOR_CAST]**   
Best Effort Neighbor-Cast: continuously resend any non-superseded messages to connected nodes until delivery is confirmed.

It should be provable that    
[REQ-STATE-GOSSIP-NEIGHBOR_CAST] +  $\Diamond\Delta$-Timely Communication ~> Termination.


Most Tendermint BFT actions are triggered when a set of messages received satisfy some criteria.
The Communication layer must therefore accumulate the messages received that might still be used to satisfy some conditions and allow Tendermint to evaluate these conditions whenever new messages are received or timeouts expire.

**[REQ-STATE-GOSSIP-KEEP_NON_SUPERSEDED]**    
For any message $m \in \text{DMsgs}[p]$ at time $t$, if there exists a time $t2 > t$ at which $m \notin \text{DMsgs}[p]$, then $\exists t1, t < t1< t2$ at which there exists a message $m1 \in \text{DMsgs}[p], m1~\text{SSS}~m$

https://github.com/tendermint/tendermint/blob/95e05b33b1ad95a88c6aac8eafc68421053bf0f2/spec/consensus/reactor/reactor.tnt#L28-L42

#### **Current implementation: Message accumulation**
The Communication layer reacts to messages by adding them to sets of similar messages, within the Communication layer internal state,  and then evaluating if Tendermint conditions are met and triggering changes to State, or by itself reacting to implement the gossip communication.
Starting a new height produces a message that supsersedes all previous messages.

### Provides to the Communication Layer
In order to identify when a message has been superseded, the State layer must provide the Communication layer with a supersession operator `SSS(lhs,rhs)`, which returns true iff $\text{lhs}$ supersedes $\text{rhs}$

#### **Current implementation: Supersession**

Currently the knowledge of message supersession is embedded in the Communication layer, which decides which messages to retransmit based on the State layer's state and the Communication layer's state.
 
Even though there is no specific superseding operator, superseding happens by advancing steps, rounds and heights.

> @josef-wider
> In the past we looked a bit into communication closure w.r.t. consensus. Roughly, the lexicographical order over the tuples (height, round, step) defines a notion of logical time, and when I am in a certain height, round and step, I don't care about messages from "the past". Tendermint consensus is mostly communication-closed in that we don't care about messages from the past. An exception is line 28 in the arXiv paper where we accept prevote messages from previous rounds vr for the same height.
> 
> I guess a precise constructive definition of "supersession" can be done along these lines.

```scala
pure def SSS(lhs,rhs): (Message, Message) => bool =
    //TODO: provide actual definition, considering the comment above
} 
```


## Communication Layer (AKA Gossip)
The communication layer provides the facilities for the State layer to communicate with other nodes by sending messages and by receiving callbacks when conditions specified in the algorithm are met.


### Requires from the State layer (Gossip-I)
Since the network is not fully connected, communication happens at a **local** level, in which information is delivered to the neighbors of a node, and a **global level**, in which information is forwarded on to neighbors of neighbors and so on.

Since connections and disconnections may happen continuously and the total membership of the system is not knowable, reliably delivering messages in this scenario would require buffering messages indefinitely, in order to pass them on to any nodes that might be connected in the future.
Since buffering must be limited, the Communication layer needs to know which messages have been superseded and can be dropped, and that the number of non-superseded messages at any point in time is bounded.


**[REQ-GOSSIP-STATE-SUPERSESSION.1]**   
`SSS(lhs,rhs)` is provided.


**[REQ-GOSSIP-STATE-SUPERSESSION.2]**    
There exists a constant $c \in Int$ such that, at any point in time, for any process $p$, the subset of messages in Msgs[p] that have not been superseded is smaller than $c$.

> **Note**    
> Supersession allows dropping messages but does not require it.

```scala
def isSupersededIn(msg, msgs): (Message, Set[Message]) => bool =
    msgs.filter(m => m.SSS(msg))

def nonSupersededMsgs(p: Proc) => Set[Message] = 
    Msgs[p].filter(m => isSupersededIn(m, Msgs[p]).not())

Int.exists(c => Proc.forall(p => always size(nonSupersededMsgs(p)) < c))
```

[REQ-GOSSIP-STATE-SUPERSESSION.2] implies that the messages being broadcast by the process itself and those being forwarded must be limited.
In Tendermint BFT this is achieved by virtue of only validators broadcasting messages and the set of validators being always limited.

Although only validators broadcast messages, even non-validators (including sentry nodes) must deliver them, because: 
* only the nodes themselves know if they are validators,
* non-validators may also need to decide to implement the state machine replication, and,
* the network is not fully connected and non-validators are used to forward information to the validators.

Non-validators that could support applications on top may be able to inform the communication layer about superseded messages (for example, upon decisions).

Non-validators that are deployed only to facilitate communication between peers (i.e., P2P only nodes, which implement the Communication layer but not the State layer) still need to be provided with a supersession operator in order to limit buffering.

#### Current implementation: P2P only nodes**
All nodes currently run Tendermint BFT, but desire to have lightweight, gossip only only, nodes has been expressed, e.g. in [ADR052](#references)

### Provides to the State layer (Gossip-I)

To broadcast as message $m$, process $p$ adds it to the set `BMsgs[p]` set.

```scala
action broadcast(p,m): (Process, Message) = 
    Msgs' = Msgs.set(p, Msgs[p].union(Set(m)))
```



**[PROV-GOSSIP-STATE-NEIGHBOR_CAST.1]**   
For any message $m$ added to BMsgs[p] at instant $t$, let NePt be the value of Ne[$p$] at time $t$; for each process $q \in \text{Ne}[p]$, $m$ will be delivered to $q$ at some point in time $t1 > t$, or there exists a point in time $t2 > t$ at which $q$ disconnects from $p$, or a message $m1$ is added to $\text{BMsgs}[p]$ at some instant $t3 > t$ and $\text{SSS}(m1,m)$.

```scala
always (
    all {
        m.in(BMsgs[p])
        q.in(Ne[p])
    } implies eventually (
        any {
            m.in(DMsgs[q]),
            BMsgs[p].exists(m1 => SSS(m1,m)),
            q.notin(Ne[p])
        }
    )
)
```

**[PROV-GOSSIP-STATE-NEIGHBOR_CAST.2]**   
For every message received, either the message itself is forwarded or a superseding message is broadcast.

```scala
always(
    (
        DMsgs[p].contains(m)
    ) implies eventually ( 
        any {
            Msgs[p].contains(m), 
            Msgs[p].exists(mn => SSS(mn,m))
        }
    )
)
```

Observe that the requirements from the State allow the Communication layer to provide these guarantees as a best effort and while bounding the memory used.

> [PROV-GOSSIP-STATE-NEIGHBOR_CAST.1] + [PROV-GOSSIP-STATE-NEIGHBOR_CAST.2] + [REQ-GOSSIP-STATE-SUPERSESSION.2] = Best effort communication + Bounded memory usage.

#### Current implementations

`broadcast`   
State does not directly broadcast messages; it changes its state and rely on the Communication layer to see the change in the state and propagate it to other nodes.

[PROV-GOSSIP-STATE-NEIGHBOR_CAST.1]   
For each of the neighbors of the node, looping go-routines continuously evaluate the conditions to send messages to other nodes.
If a message must be sent, it is enqueued for transmission using TCP and will either be delivered to the destination or the connection will be dropped.
New connections reset the state of Communication layer wrt the newly connected node (in case it is a reconnection) and any messages previously sent (but possibly not delivered) will be resent if the conditions needed apply. If the conditions no longer apply, it means that the message has been superseded and need no be retransmitted.

[PROV-GOSSIP-STATE-NEIGHBOR_CAST.2]   
Messages delivered either cause the State to be advanced, causing the message to be superseded, or are added to the Communication layer internal state to be checked for matching conditions in the future.
From the internal state it will affect the generation of new messages, which may have exactly the same contents of the original one or not, either way superseding the original.

### Requires from P2P (P2P-I)

The P2P layer must expose functionality to allow 1-1 communication at the Communication layer, for example to implement request/response (e.g., "Have you decided?").

**[REQ-GOSSIP-P2P-UNICAST.1]**   
Ability address messages to a single neighbor.

```scala
unicast(p,q,m): (Proc,Proc,Message) =
    all {
        Ne[p].contains(q),
        UMsgs' = UMsgs.set(p, UMsgs[p].union(Set((q,m)))
    }
```

**[REQ-GOSSIP-P2P-UNICAST.2]**   
Requirement for the unicast message to be delivered.

> **TODO**
> Is this a real requirement? If it is, is [PROV-GOSSIP-STATE-NEIGHBOR_CAST.1] a real requirement?
> How different are these 2?

```scala
always (
    all {
        m.in(UMsgs[p])
        q.in(Ne[p])
    } implies eventually (
        any {
            m.in(DMsgs[q]),
            UMsgs[p].exists((q,m1) => SSS(m1,m)),
            q.notin(Ne[p])
        }
    )
)
```

**[REQ-GOSSIP-P2P-NEIGHBOR_ID]**    
Ability to discern sources of messages received.

> **TODO**
> How to specify that something WAS true?


Moreover, since the Communication layer must provide 1-to-many communication, the P2P layer must provide:

**[REQ-GOSSIP-P2P-CONCURRENT_CONN]**    
Support for connecting to multiple nodes concurrently.
```scala
assume _ = Proc.forall(p => size(Ne[p]) >= 0)
```

**[REQ-GOSSIP-P2P-CHURN-DETECTION]**    
Support for tracking connections and disconnections from neighbors.


**[REQ-GOSSIP-P2P-NON_REFUTABILITY]**     
Needed for authentication.


#### Current implementations

[REQ-GOSSIP-P2P-UNICAST]   
- `Send(Envelope)`/`TrySend(Envelope)`
    - Enqueue and forget. 
    - Disconnection and reconnection causes drop from queues.
    - Enqueuing may block for a while in `Send`, but not on `TrySend`

[REQ-GOSSIP-P2P-NEIGHBOR_ID]
- Node cryptographic IDs.
- IP Address

[REQ-GOSSIP-P2P-CONCURRENT_CONN]    
- Inherited from the network stack
- Driven by PEX and config parameters

[REQ-GOSSIP-P2P-CHURN-DETECTION]    
- `AddPeer`
- `RemovePeer`


[REQ-GOSSIP-P2P-NON_REFUTABILITY]    
- Cryptographic signing and authentication.




#### Non-requirements
- Non-duplication
    - Gossip itself can duplicate messages, so the State layer must be able to handle them, for example by ensuring idempotency.



## References
- [The latest gossip on BFT consensus](https://arxiv.org/abs/1807.0493)
- [ADR 052: Tendermint Mode](https://github.com/tendermint/tendermint/blob/master/docs/architecture/adr-052-tendermint-mode.md)
