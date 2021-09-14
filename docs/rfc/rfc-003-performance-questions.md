# RFC 003: Taxonomy of potential performance issues in Tendermint 

## Changelog

- 2021-09-02: Created initial draft (@wbanfield)

## Abstract

This document discusses the various sources of performance issue in Tendermint and
attempts to clarify what work may be required to understand and address them.

## Background

Performance, loosely defined as the ability of a software process to perform its work
quickly and efficiently under load and within reasonable resource limits, is a frequent
topic of discussion in the Tendermint project.
To effectively address any issues with Tendermint performance we need to
categorize the various issues, understand their potential sources, and gauge their
impact on users.

Categorizing the different known performance issues will allow us to discuss and fix them
more systematically. This document proposes a rough taxonomy of performance issues
and highlights areas where more research into potential performance problems is required.

Understanding Tendermint's performance limitations will also be critically important
as we make changes to many of its subsystems. Performance is a central concern for
upcoming decisions regarding the `p2p` protocol, RPC message encoding and structure,
database usage and selection, and consensus protocol updates.


## Discussion

This section attempts to delineate the different sections of Tendermint functionality
that are often cited as having performance issues. It raises questions and suggests
lines of inquiry that may be valuable for better understanding Tendermint's performance issues.

As a note: We should avoid quickly adding many microbenchmarks or package level benchmarks. 
These are prone to being worse than useless as they can obscure what _should_ be
focused on: performance of the system from the perspective of a user. We should,
instead, tune performance with an eye towards actions user needs. These users comprise
both operators of Tendermint chains and the people generating transactions for
Tendermint chains. Both of these sets of users are largely aligned in wanting an end-to-end
system that operates quickly and efficiently.

REQUEST: The list below may be incomplete, if there are additional sections that are often
cited as creating poor performance, please comment so that they may be included.

### P2P

#### Tendermint cannot scale to large numbers of nodes

A complaint has been reported that Tendermint networks cannot scale to large numbers of nodes.
The listed number of nodes a user reported as causing issue was in the thousands.
We don't currently have evidence about what the upper-limit of nodes that Tendermint's
P2P stack can scale to.

We need to more concretely understand the source of issue and determine what layer
is causing a problem. It's possible that the P2P layer, in the absence of any reactors
sending data, is perfectly capable of managing thousands of peer connections. For
a reasonable networking and application setup, thousands of connections should not present any
issue for the application.

We need more data to understand the problem directly. We want to drive the popularity
and adoption of Tendermint and this will mean allowing for chains with more validators.
We should follow up with users experiencing this issue. We may then want to add
a series of metrics to the P2P layer to better understand the inefficiencies it produces.

The following metrics can help us understand the sources of latency in the Tendermint P2P stack:

* Number of messages sent and received per second
* Time of a message spent on the P2P layer send and receive queues

The following metrics exist and should be leveraged in addition to those added:

* Number of peers node's connected to
* Number of bytes per channel sent and received from each peer

### Sync

#### Block Syncing is slow

Bootstrapping a new node in a network to the height of the rest of the network
takes longer than users would like. Block sync requires fetching all of the blocks from
peers and placing them into the local disk for storage. A useful line of inquiry
is understanding how quickly a perfectly tuned system _could_ fetch all of the state
over a network so that we understand how much overhead Tendermint actually adds.

The operation is likely to be _incredibly_ dependent on the environment in which
the node is being run. The factors that will influence syncing include:
1. Number of peers that a syncing node may fetch from.
2. Speed of the disk that a validator is writing to.
3. Speed of the network connection between the different peers that node is
syncing from.

We should calculate how quickly this operation _could possibly_ complete for common chains and nodes.
To calculate how quickly this operation could possibly complete, we should assume that
a node is reading at line-rate of the NIC and writing at the full drive speed to its
local storage. Comparing this theoretical upper-limit to the actual sync times
observed by node operators will give us a good point of comparison for understanding
how much overhead Tendermint incurs.

We should additionally add metrics to the blocksync operation to more clearly pinpoint
slow operations. The following metrics should be added to the block syncing operation:

* Time to fetch and validate each block
* Time to execute a block
* Blocks sync'd per unit time

### Application

Applications performing complex state transitions have the potential to bottleneck
the Tendermint node.

#### ABCI block delivery could cause slowdown

ABCI delivers blocks in several methods: `BeginBlock`, `DeliverTx`, `EndBlock`, `Commit`.

Tendermint delivers transactions one-by-one via the `DeliverTx` call. Most of the 
transaction delivery in Tendermint occurs asynchronously and therefore appears unlikely to
form a bottleneck in ABCI.

After delivering all transactions, Tendermint then calls the `Commit` ABCI method.
Tendermint [locks all access to the mempool][abci-commit-description] while `Commit` 
proceeds. This means that an application that is slow to execute all of its 
transactions or finalize state during the `Commit` method will prevent any new 
transactions from being added to the mempool.  Apps that are slow to commit will 
prevent consensus from proceeded to the next consensus height since Tendermint 
cannot validate validate block proposals or produce block proposals without the 
AppHash obtained from the `Commit` method.

#### ABCI serialization overhead causes slowdown

The most common way to run a Tendermint application is using the Cosmos-SDK.
The Cosmos-SDK runs the ABCI application within the same process as Tendermint.
When an application is run in the same process as Tendermint, a serialization penalty
is not paid. This is because the local ABCI client does not serialize method calls
and instead passes the protobuf type through through directly. This can be seen
in [local_client.go][abci-local-client-code].

Serialization and deserialization in the gRPC and socket protocol ABCI methods
may cause slowdown. While these may cause issue, they are not part of the primary
usecase of Tendermint and do not necessarily need to be addressed at this time.

### RPC

#### The Query API

* Not yet clear what the use case is for this or if this is the best solve for the need for users to query TM state? 
* Should we solve this within Tendermint?

#### RPC Serialization

The Tendermint RPC uses a modified version of JSON-RPC. This RPC powers the `broadcast_tx_*` methods,
which is a critical method for adding transactions to Tendermint at the moment. This method is
likely invoked quite frequently on popular networks. Being able to perform efficiently
on this common and critical operation is very important. The current JSON-RPC implementation
relies heavily on type introspection via reflection, which is known to be very slow in
Go. We should therefore produce benchmarks of this method to determine how much overhead
we are adding to what, is likely to be, a very common operation.

The other JSON-RPC methods are much less critical to the core functionality of Tendermint.
While there may other points of performance consideration within the RPC, methods that do not
receive high volumes of requests should not be prioritized for performance consideration.

NOTE: Previous discussion of the RPC framework was done in [ADR 57][adr-57] and 
there is ongoing work to inspect and alter the JSON-RPC framework in [RFC 002][rfc-002]. 
Much of these RPC-related performance considerations can either wait until the work of RFC 002 work is done or be
considered concordantly with the in-flight changes to the JSON-RPC.

### Protocol

#### Gossiping messages

Currently, for any validator to successfully vote in a consensus _step_, it must
receive votes from greater than 2/3 of the validators on the network. In many cases,
it's preferable to receive as many votes as possible from correct validators.

This produces a quadratic increase in messages that are communicated as more validators join the network.
(Each of the N validators must communicate with all other N-1 validators).

This large number of messages communicated per step has been identified to impact
performance of the protocol. Given this that number of messages communicated has been
identified as a bottleneck, it would be extremely valuable to gather data on how long
it takes for popular chains with many validators to gather all votes within a step.

Metrics that would improve visibility into this include:

* Amount of time for a node to gather votes in a step.
* Number of votes each node sends to gossip (i.e. not its own votes, but votes it is
transmitting for a peer).
* Total number of votes each node sends to receives (A node may receive duplicate votes
so understanding how frequently this occurs will be valuable in evaluating the performance
of the gossip system).

#### Tx hashes

Using a faster hash algorithm for Tx hashes is currently a point of discussion
in Tendermint. Namely, it is being considered as part of the [modular hashing proposal][modular-hashing].
It is currently unknown if hashing transactions in the Mempool forms a significant bottleneck.
Although it does not appear to be documented as slow, there are a few open github
issues that indicate a possible user preference for a faster hashing algorithm,
including [issue 2187][issue-2187] and [issue 2186][issue-2186]. 

It is likely worth investigating what order of magnitude Tx hashing takes in comparison to other
aspects of adding a Tx to the mempool. It is not currently clear if the rate of adding Tx
to the mempool is a source of user pain. We should not endeavor to make large changes to
consensus critical components without first being certain that the change is highly
valuable and impactful.

### Digital Signatures

#### Verification

Working with cryptographic signatures can be computationally expensive. The cosmos
hub uses [ed25519 signatures][hub-signature]. The library performing signature
verification in Tendermint on votes is [benchmarked][ed25519-bench] to be able to perform an `ed25519`
signature in 75μs on a decently fast CPU. A validator in the Cosmos Hub performs
3 sets of verifications on the signatures of the other 139 validators in the Hub
in a consensus round, during block verification, when verifying the prevotes, and
when verifying the precommits. With no batching, this would be roughly `3ms` per
round. It is quite unlikely, therefore, that this accounts for any serious amount
of the ~7 seconds of block time per height in the Hub.

#### Use in gossip protocol

Currently, Tendermint's digital signature verification requires that all validators
receive all vote messages. Each validator must receive the complete digital signature
along with the vote message that it corresponds to. This means that all N validators
must receive messages from at least 2/3 of the N validators in each consensus
round. Given the potential for oddly shaped network topologies and the expected
variable network roundtrip times of a few hundred milliseconds in a blockchain,
it is highly likely that this amount of gossiping is leading to a significant amount
of the slowdown in the Cosmos Hub and in Tendermint consensus.

### Tendermint Event System

// TODO: (@wbanfield)

### References
[modular-hashing]: https://github.com/tendermint/tendermint/pull/6773
[issue-2186]: https://github.com/tendermint/tendermint/issues/2186
[issue-2187]: https://github.com/tendermint/tendermint/issues/2187
[rfc-002]: https://github.com/tendermint/tendermint/pull/6913
[adr-57]: https://github.com/tendermint/tendermint/blob/master/docs/architecture/adr-057-RPC.md
[issue-1319]: https://github.com/tendermint/tendermint/issues/1319
[abci-commit-description]: https://github.com/tendermint/spec/blob/master/spec/abci/apps.md#commit
[abci-local-client-code]: https://github.com/tendermint/tendermint/blob/511bd3eb7f037855a793a27ff4c53c12f085b570/abci/client/local_client.go#L84
[hub-signature]: https://github.com/cosmos/gaia/blob/0ecb6ed8a244d835807f1ced49217d54a9ca2070/docs/resources/genesis.md#consensus-parameters
[ed25519-bench]: https://github.com/oasisprotocol/curve25519-voi/blob/d2e7fc59fe38c18ca990c84c4186cba2cc45b1f9/PERFORMANCE.md
