# ADR D001: Inter validator set messaging


## Changelog <!-- omit in toc -->

* 2021-09-29: Initial version of the document
    
## Table of contents <!-- omit in toc -->
 
1. [Context](#context)
   1. [Definitions](#definitions)
   2. [Problem statement](#problem-statement)
   3. [Solution](#solution)
2. [Alternative Approaches](#alternative-approaches)
   1. [Rotate on Tenderdash](#rotate-on-tenderdash)
   2. [TODO](#todo)
3. [Decision](#decision)
4. [Detailed Design](#detailed-design)
   1. [What are the user requirements?](#what-are-the-user-requirements)
   2. [What systems will be affected?](#what-systems-will-be-affected)
   3. [What new data structures are needed, what data structures need changes?](#what-new-data-structures-are-needed-what-data-structures-need-changes)
   4. [What new APIs will be needed, what APIs will change?](#what-new-apis-will-be-needed-what-apis-will-change)
   5. [What are the efficiency considerations (time/space)?](#what-are-the-efficiency-considerations-timespace)
   6. [What are the expected access patterns (load/throughput)?](#what-are-the-expected-access-patterns-loadthroughput)
   7. [Are there any logging, monitoring, or observability needs?](#are-there-any-logging-monitoring-or-observability-needs)
   8. [Are there any security considerations?](#are-there-any-security-considerations)
   9. [Are there any privacy considerations?](#are-there-any-privacy-considerations)
   10. [How will the changes be tested?](#how-will-the-changes-be-tested)
   11. [How will the changes be broken up for ease of review?](#how-will-the-changes-be-broken-up-for-ease-of-review)
   12. [Will these changes require a breaking (major) release?](#will-these-changes-require-a-breaking-major-release)
   13. [Does this change require coordination with the SDK or any other software?](#does-this-change-require-coordination-with-the-sdk-or-any-other-software)
5. [Status](#status)
6. [Consequences](#consequences)
   1. [Positive](#positive)
   2. [Negative](#negative)
   3. [Neutral](#neutral)
7. [References](#references)

## Context

### Definitions

For the needs of this document, we define the following types of nodes:

* Validator nodes, which participate in the consensus protocol, verify and sign blocks, 
* Full nodes, which execute valid blocks.

Note: there are also light nodes, but light nodes are not relevant for the discussion.

Validator Set is a group of Validator nodes responsible for the consensus at a given time. Only one Validator Set can
be active at the given time. After a predefined number of blocks, the ABCI app initiates the Validator Set rotation to
make another Validator Set active. 

### Problem statement

The consensus process requires direct communication between validators, as full nodes do not accept nor propagate
consensus protocol messages (like votes).

Every node, regardless of its type, selects peer nodes they connect to on a random basis. There is no differentiation
between Full and Validator nodes, and there is no method to ensure that each Validator is (directly or through other
Validators) connected to all other Validators. In an extreme case, a Validator node can be connected only to Full
nodes, effectively blocking its communication and excluding it from participation in the consensus.

### Solution

Each Validator node shall allocate a preconfigured number of connectivity slots for communication with other Validators
that belong to the same Validator Set.

In the current implementation, the ABCI application manages the rotation of Validator Sets. When rotation is needed, 
the application sends information about the new Validator Set in response to the `EndBlock` message. 

`EndBlock` message shall be extended by adding a network address of each Validator node. Based on that, each
member of the active Validator Set will connect with a predefined number of Peer Validators belonging to the same 
Validator Set. A list of Peer Validators will be determined using the algorithm described in 
[DIP 0006](https://github.com/dashpay/dips/blob/master/dip-0006.md#building-the-set-of-deterministic-connections).

This solution introduces a risk of delayed propagation of new block due to limited connection slots between Full and
Validator nodes. To mitigate this risk, we can tune the number of Full node connection slots independently from
Validator node connection slots.

## Alternative Approaches

### Rotate on Tenderdash

ABCI application can send the whole master node list and quorum list to Tenderdash and
potentially rotate on its side. However, it would split the responsibility for the
management of Validator Set rotation between multiple components. It would also require
much more additional implementation. It would be better to keep the current Tendermint
design and concept being a logic-less consensus library and keep all business logic in
the ABCI application.

### TODO

## Decision

> TODO: This section records the decision.
> It is best to record as much info as possible from the discussion that happened. This aids in not having to go back
> to the Pull Request to get the needed information.

## Detailed Design

### What are the user requirements?

Having a network with much more Full nodes than Validators ensure each Validator Node can connect (directly or
through another Validator) to any other Validator which is a member of the same active Validator Set.

### What systems will be affected?

* Tenderdash
* ABCI App

### What new data structures are needed, what data structures need changes?

1. Separate Validator Set member connection pool from Full node connection pool
2. Additional tuning parameters:
    1. Full node connection slots
    2. Validator node connection slots

### What new APIs will be needed, what APIs will change?

In the ABCI protocol, we add a network address of each member of the active Validator Set to the
`ResponseEndBlock.ValidatorSetUpdate.ValidatorUpdate` structure. The network address structure is as follows:

```protobuf
enum NetAddressType {
  IPv4 = 0;
};

message NetAddress {
  reserved 1; // Reserved for compatibility with tendermint.p2p.NetAddress
  reserved "id"; // Reserved for compatibility with tendermint.p2p.NetAddress
  string address = 2;
  uint32 port = 3;
  NetAddressType address_type = 4;
}
```

### What are the efficiency considerations (time/space)?

1. This change will increase the amount of data sent by the ABCI application to Tenderdash. The connection between them
is a local Unix socket. Therefore the change should not have any noticeable performance or bandwidth impact.
2. Improved consensus performance thanks to a shorter and more predictable distance between each Validator in a
Validator Set.

### What are the expected access patterns (load/throughput)?

1. Each rotation of the active Validator Set will cause the ABCI application to send additional data to Tenderdash. 
   The number of blocks between rotation events is a configuration parameter of the ABCI application.

### Are there any logging, monitoring, or observability needs?

1. Operator shall be able to determine a list of node peers, together with their type (Full, inactive Validator, active 
Validator) and connection status. This information can be part of debug logs.

### Are there any security considerations?

This change makes connectivity between Validators more predictable.
It can make it a bit easier for a malicious user to deliberately block communication
for a given Validator node to make it unable to participate in the Validator Set.

However, this risk is already present at the ABCI application level, so the change
does not introduce new risks.

### Are there any privacy considerations?

This change does not impact user privacy.

### How will the changes be tested?

Testing strategy for this change involves:

1. Reproduction of the problem statement as an end-to-end (e2e) test, with a network of much many more full nodes than
   validator nodes.
2. Implementation of unit tests for the new code
3. Execution of e2e tests to confirm the issue is not reproduced anymore
4. Starting a long-living end-to-end cluster (at least a few days of operation) and monitoring of the outcome
5. Deployment to the test net

### How will the changes be broken up for ease of review?

This change consists of the following tasks:

1. Implementation of end-to-end tests to reproduce the problem statement
   1. Reproduce "orphaned" Validator Set member scenario
2. Enhancement of the ABCI protocol:
   1. Addition of new field on Tenderdash
   2. Update of end-to-end tests to reflect ABCI protocol changes
   3. Implementation of ABCI protocol changes inside the ABCI application
3. Implementation of dedicated slots for Intra Validator Set connectivity within Tenderdash
4. Tenderdash reactions on Validator Set rotation events:
   1. Storing and updating Validator Set members network addresses
   2. Establishing connections within Validator Sets
   3. Unit tests that emulate and validate Inter Validator Set communication
5. Implementation of end-to-end tests to verify Full node "starving" scenarios

### Will these changes require a breaking (major) release?

As the change introduces a new required field into the ABCI app, it requires a new breaking release.

### Does this change require coordination with the SDK or any other software?

The change requires coordination with: 

1. ABCI application
2. TODO

## Status

> {Deprecated|Proposed|Accepted|Declined}

Proposed

## Consequences

> TODO: This section describes the consequences after applying the decision. All consequences should be summarized
> here, not just the "positive" ones.

### Positive

1. Improved stability of active Validator Set operations.

### Negative

1. Additional complexity of ABCI protocol
2. Additional changes compared to the upstream Tendermint project make backporting harder.

### Neutral

## References

> TODO: Are there any relevant PR comments, issues that led up to this, or articles referenced for why we made the given
> design choice? If so, link them here!

* [Dash Core Group Release Announcement: Dash Platform v0.20 on Testnet](https://blog.dash.org/dash-core-group-release-announcement-dash-platform-v0-20-on-testnet-c8fa00d28af7)
* [DIP 0006: Long Living Masternode Quorums (LLMQ)](https://github.com/dashpay/dips/blob/master/dip-0006.md)
* {reference link}
