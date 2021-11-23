# Testing Tendermint

This document summarizes the changes made to the tendermint implementation and the test scenarios that we were able to test. The tendermint nodes have to be run independently and configured to talk to the **Scheduler**.

## Changes to the Implementation

Tendermint version: **0.34.3**

Repository: https://github.com/zeu5/tendermint/tree/pct-instrumentation

We had to first change the implementation to transmit messages to **Scheduler** as opposed to sending it to the intended replica directly. This required us to implement the `Transport` and `Connection` interfaces of the `p2p` module which are used for communication.

We implemented `InterceptedTransport` and `InterceptedConnection` ([link to code](https://github.com/zeu5/tendermint/blob/pct-instrumentation/p2p/transport_intercept.go)).

- `InterceptedTransport` wraps around `MConnTransport` to help create the connection object
- `InterceptedConnection` wraps around `mConnConnection` which is used for handshake purposes. The `ReceiveMessage` and `SendMessage` methods were overridden to receive from and send to the **Scheduler**. This was achieved using the [client library](https://github.com/ds-test-framework/go-clientcontroller)

Additionally, running multiple test cases requires the nodes to start, stop and restart using RPC calls. For this we created a new struct [`NodeManager`](https://github.com/zeu5/tendermint/blob/pct-instrumentation/node/node_manager.go) in the `node` module. `NodeManager` instantiates the transport and passes it to `Node`. On restart, `NodeManager` creates a new `Node` object using the same transport instance.

## Test cases

### Implemented and executed

The logs for the following testcases can be found [here](https://github.com/ds-test-framework/tendermint-test/tree/master/logs).

1. We create test scenarios for skipping rounds ([1](./testcases/roundskip1.md), [2](./testcases/roundskip2.md))
2. We test the behaviour when the replicas lock onto a particular proposal ([1](./testcases/lockedvalue1.md), [2](./testcases/lockedvalue2.md))
3. We test scenarios when the replica receives a different proposal and check its `Prevote` ([1](./testcases/prevoting.md))

### Deviations from protocol

1. The implementation provides the ability for a replica to unlock a block that has already been locked.
2. The implementation does not behave as per the protocol when it receives a proposal with a different valid round.
3. The implementation does not change the `Prevote` based on the value proposed. It currently `Prevote`s the locked block, if it has locked irrespective of the proposal.
4. The implementation does not force a replica to move to a higher round if it receives `f+1` messages from a higher round. The implementation forces the replica to move to a higher round only when it sees `2f+1` votes of a higher round.
5. We also speculate that the implementation allows for behaviours when a replica can change its vote.
    In the current implementation, there are optimizations done to count votes and allows for replicas to claim majority. The flags used to record these claims are also used when checking for duplicity of votes, allowing a replica that has sent a vote to change it.

### Further planned testcases

We have more testcases planned to test specific parts of the protocol. Together, the testcases should serve as a concrete example set which will aid the developers to write testcases.

- Testing the relation of a block timestamp to real global time.

    There are interesting scenarios where we can engineer the timestamp on a block that will be committed. There are changes proposed to this mechanism and we would like to create test scenarios where we can test the difference in behaviour between the two versions of the protocol.

- No progress without enough votes

    We know that replicas don't make progress if they do not receive `2f+1` votes. We would like to test if replicas are able to move to higher rounds or commit even when `2f+1` votes have not been delivered.

- Testing scenarios of vote counting

    We suspect there are differences in the implementation and the specification which allow for interesting behaviours. Specifically, the way votes are counted.
