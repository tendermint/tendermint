# ADR D001: Inter validator set messaging

## Changelog <!-- omit in toc -->

* 2021-09-29: Initial version of the document
* 2021-10-05: Clarification of the document based on peer review
* 2021-10-22: Updated document based on implementation
* 2022-01-05: Review after implementation is done

## Table of contents <!-- omit in toc -->
 
1. [Context](#context)
   1. [Definitions](#definitions)
   2. [Problem statement](#problem-statement)
2. [Alternative Approaches](#alternative-approaches)
   1. [Scenario 1: Trigger rotation from ABCI app passing addresses of all quorum members](#scenario-1-trigger-rotation-from-abci-app-passing-addresses-of-all-quorum-members)
   2. [Scenario 2: Trigger rotation from ABCI app passing quorum id to rotate](#scenario-2-trigger-rotation-from-abci-app-passing-quorum-id-to-rotate)
   3. [Scenario 3: Rotate on Tenderdash](#scenario-3-rotate-on-tenderdash)
3. [Decision](#decision)
4. [Detailed Design](#detailed-design)
   1. [What are the user requirements?](#what-are-the-user-requirements)
   2. [What systems will be affected?](#what-systems-will-be-affected)
   3. [What new data structures are needed, what data structures need changes?](#what-new-data-structures-are-needed-what-data-structures-need-changes)
   4. [What new APIs will be needed, what APIs will change?](#what-new-apis-will-be-needed-what-apis-will-change)
      1. [ABCI Protocol](#abci-protocol)
      2. [P2P Handshake](#p2p-handshake)
   5. [What are the efficiency considerations (time/space)?](#what-are-the-efficiency-considerations-timespace)
   6. [What are the expected access patterns (load/throughput)?](#what-are-the-expected-access-patterns-loadthroughput)
   7. [Are there any logging, monitoring, or observability needs?](#are-there-any-logging-monitoring-or-observability-needs)
   8. [Are there any security considerations?](#are-there-any-security-considerations)
   9. [Are there any privacy considerations?](#are-there-any-privacy-considerations)
   10. [How will the changes be tested?](#how-will-the-changes-be-tested)
   11. [How will the changes be broken up for ease of review?](#how-will-the-changes-be-broken-up-for-ease-of-review)
   12. [Will these changes require a breaking (major) release?](#will-these-changes-require-a-breaking-major-release)
   13. [Does this change require coordination with the SDK or any other software?](#does-this-change-require-coordination-with-the-sdk-or-any-other-software)
   14. [Future work](#future-work)
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
* Full nodes, which verify and execute blocks. Full nodes verify the recovered threshold signature of the block
commit message against the current validator set's quorum threshold public key.

Note: there are also light nodes, but they are not relevant for this discussion.

A Validator Set is a group of Validator nodes responsible for driving consensus at any given time. For each block, one
of the validator nodes is a proposer. Each other validator votes on the validity of the block.

Only one Validator Set can be active at the given time. After a predefined number of blocks, the ABCI app initiates the Validator Set rotation to make another Validator Set active. 

### Problem statement

The consensus process requires direct communication between Validators, as Full nodes do not know the public keys of
Validators and therefore can neither verify nor propagate any consensus protocol message individually signed by these
Validators. It means that only Validators can propagate votes. However, Full nodes can propagate the final commit
message, which instead contains the recovered threshold signature of the pre-commit voting round. Full nodes can perform
this verification because they know the Validator Set's quorum threshold public key.

Each node (regardless of its type) randomly selects peer nodes to connect. There is no differentiation
between Full and Validator nodes, and there is no method to ensure that each Validator is (directly or through other
Validators) connected to some other Validators. In an extreme case, a Validator node can be connected only to Full
nodes, effectively blocking all inbound and outbound block proposals and voting, hence excluding it from
participation in the consensus. It is also possible that Validator nodes form "islands" where a few Validators are
connected between themselves but then are separated from other Validator nodes by full nodes.

## Alternative Approaches

### Scenario 1: Trigger rotation from ABCI app passing addresses of all quorum members

During Validator Set rotation, each Validator node shall connect to a predefined number of other Validators that belong to the same Validator Set.

In the current implementation, the ABCI application manages the rotation of Validator Sets. When rotation is needed, 
the application sends information about the new Validator Set in response to the `EndBlock` message. 

`EndBlock` message shall be extended by adding a network address of each Validator node. Based on that, each
member of the active Validator Set will connect with a predefined set of Peer Validators belonging to the same 
Validator Set. A list of Peer Validators will be determined using the algorithm described in 
[DIP 0006](https://github.com/dashpay/dips/blob/master/dip-0006.md#building-the-set-of-deterministic-connections).

This solution introduces a risk of delayed propagation of new block due to limited connection slots between Full and
Validator nodes. To mitigate this risk, we can implement the number of Full node connection slots independently from
Validator node connection slots.

### Scenario 2: Trigger rotation from ABCI app passing quorum id to rotate

This scenario is very similar to Scenario 1. However, instead of passing addresses of all Validators that are members of
a Validator Set, only quorum ID is passed.

This solution requires Tendermint to determine Validators addresses, effectively re-implementing logic already done in
the ABCI App.

### Scenario 3: Rotate on Tenderdash

ABCI application can send the whole master node list and quorum list to Tenderdash and
potentially rotate on its side. However, it would split the responsibility for
Validator Set rotation between multiple components. It would also require
much more additional implementation. It would be better to keep the current Tendermint
design and concept being a logic-less consensus library and keep all business logic in
the ABCI application.

## Decision

After discussion, we will implement the solution described in **Scenario 1**. It will allow us to:

* reuse logic already implemented as part of ABCI App,
* keep Tenderdash a logic-less consensus library,
* keep single responsibility principle.

## Detailed Design

### What are the user requirements?

Having a network with much more Full nodes than Validators ensure each Validator Node can connect (directly or
through another Validator) to any other Validator which is a member of the same active Validator Set.

### What systems will be affected?

* Tenderdash
* ABCI App (Drive)

### What new data structures are needed, what data structures need changes?

1. New package `dash` to group Tenderdash-specific code
2. `dashtypes.ValidatorAddress` representing an address of a validator received from the ABCI app
3. Introduced several internal helper data structures (`sortableValidator`, `sortedValidatorList`, `validatorMap`)

### What new APIs will be needed, what APIs will change?

#### ABCI Protocol

In the ABCI protocol, we add a network address of each member of the active Validator Set to the
`ResponseEndBlock.ValidatorSetUpdate.ValidatorUpdate` structure. The network address should be a URI:

```uri
tcp://<node_id>@<address>:<port>
```

where:

* `<node_id>` - node ID (can be generated based on node P2P public key, see [p2p.PubKeyToID](https://github.com/dashevo/tenderdash/blob/20d1f91c0adb29d1b5d03a945b17513a368759e4/p2p/key.go#L45))
* `<address>` - IP address of the validator node
* `<port>` - p2p port number of the validator node

The `ValidatorUpdate` structure looks as follows:

```protobuf
message ValidatorUpdate {
  tendermint.crypto.PublicKey pub_key      = 1 [(gogoproto.nullable) = true];
  int64                       power        = 2;
  bytes                       pro_tx_hash  = 3;
  string                      node_address = 4 [(gogoproto.nullable) = true];  // address of the Validator
}
```


If the ABCI app cannot determine `<node_id>`, it can leave it empty. In this case, Tenderdash will try to retrieve a public
key from its address book or remote host and derive node ID from it, as described below.

#### P2P Handshake

To retrieve node ID from a **remote node**, **local node** can establish a short-lived connection to the **remote node** to download its public key. This feature does not require any changes to the P2P protocol; it just uses existing capabilities.

A public key request starts with a standard flow Diffie-Hellman protocol, as implemented in `conn.MakeSecretConnection()`. Once ephemeral keys are exchanged between nodes, **local node** proceeds to  `shareAuthSignature()`. However, instead of sending its own `AuthSigMessage`, it waits for **remote node**'s auth signature to arrive. As the communication is asynchronous, the **remote node** sends its `AuthSigMessage`. That is when the **local node** closes the connection.

This algorithm introduces a potential node ID spoofing risk, discussed later.

### What are the efficiency considerations (time/space)?

1. This change will increase the amount of data sent by the ABCI application to Tenderdash. The connection between them is local. Therefore the change should not have any noticeable performance or bandwidth impact.
2. Improved consensus performance thanks to a shorter and more predictable distance between each Validator in a Validator Set.

In the future, additional optimizations can further improve the performance, for example:

1. use Unix sockets for communication between Tenderdash and ABCI App,
2. use shared memory to speed up inter-process communication; for instance, ABCI should keep (signed) information about active validator-set in shared memory and send only a trigger to take them from there.


### What are the expected access patterns (load/throughput)?

1. Each rotation of the active Validator Set will cause the ABCI application to send additional data to Tenderdash. The number of blocks between rotation events is a configuration parameter of the ABCI application.
2. Each Validator in the active validator set will establish a few more tcp connections.
3. Retrieving public keys for validators can be resource and time-intensive, as it establishes new connections to validators and performs a DH handshake. This issue is mitigated by:
   a. ensuring that this feature is used only when needed
   b. refactoring and simplification of the P2P protocol and source code used to retrieve public keys

   Note that for the needs of Inter-Validator Set Messaging, retrieving public keys is executed only for `log2(n-1)` Validators, where `n` is size of active Validator Set.
   
### Are there any logging, monitoring, or observability needs?

1. Operator can determine a list of node peers, together with their type (Full, inactive Validator, active Validator) and connection status. This information can be part of debug logs.

### Are there any security considerations?

1. This change makes connectivity between Validators more predictable.
It can make it a bit easier for a malicious user to block communication
for a given Validator node to make it unable to participate in the Validator Set.
However, this risk is already present at the ABCI application level, so the change
does not introduce new risks.
2. Missing node ID can be determined based on a public key, retrieved without proper validation. It cannot be guaranteed that it will not be spoofed. 

   That can allow an attacker who controls the network connectivity to perform a man-in-the-middle attack. As the attacker does not have the Validator's private keys, he/she cannot perform unauthorized operations. However, it is possible to intercept communication and isolate the attacked Validator from the quorum. In the future, additional validation of the Validator's private key based on proTxHash can help mitigate this risk.
   
   If the attacker controls the network connectivity, he can still drop that traffic on the network level.

  

### Are there any privacy considerations?

This change does not impact user privacy.

### How will the changes be tested?

Testing strategy for this change involves:

1. Reproduction of the problem statement.
2. Implementation of unit tests for the new code
3. Starting a long-living end-to-end cluster (at least a few days of operation) and monitoring of the outcome
4. Deployment to the test net

### How will the changes be broken up for ease of review?

This change consists of the following tasks:

1. Enhancement of the ABCI protocol:
   1. Addition of new field on Tenderdash
   2. Update of end-to-end tests to reflect ABCI protocol changes
   3. Implementation of ABCI protocol changes inside the ABCI application
2. Tenderdash reactions on Validator Set rotation events:
   1. Storing and updating Validator Set members network addresses
   2. Establishing connections within Validator Sets
   3. Unit tests that emulate and validate Inter Validator Set implementation

### Will these changes require a breaking (major) release?

As the change introduces a new required field into the ABCI app, it requires implementation on the ABCI App side.
However, this feature shall be backward-compatible.

### Does this change require coordination with the SDK or any other software?

The change requires coordination with the ABCI application.

### Future work

1. Implement additional validation of the Validator's private key based on proTxHash.
2. Extend service data in Deterministic Masternode Lists (DIP-3) to keep node ID, p2p port, and other platform information. It will allow us to remove the handshake process described in "Security considerations" above.

## Status

> {Deprecated|Proposed|Accepted|Declined}

Accepted

## Consequences

### Positive

1. Improved stability of active Validator Set operations.
2. Quicker Voting rounds.

### Negative

1. Additional complexity of ABCI protocol
2. Additional changes compared to the upstream Tendermint project make backporting harder.

### Neutral

## References

* [Dash Core Group Release Announcement: Dash Platform v0.20 on Testnet](https://blog.dash.org/dash-core-group-release-announcement-dash-platform-v0-20-on-testnet-c8fa00d28af7)
* [DIP 0006: Long Living Masternode Quorums (LLMQ)](https://github.com/dashpay/dips/blob/master/dip-0006.md)
* [DIP 0003: Deterministic Masternode Lists (DML)](https://github.com/dashpay/dips/blob/master/dip-0003.md)
