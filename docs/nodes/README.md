---
order: 1
parent:
  title: Node Operators
  order: 4
---

# Overview

This section will focus on how to operate full nodes, validators and light clients.

- [Node Types](#node-types)
- [Configuration](./configuration.md)
  - [Configure State sync](./state-sync.md)
- [Validator Guides](./validators.md)
  - [Running in Production](./running-in-production.md)
  - [How to secure your keys](./validators.md#validator_keys)
  - [Remote Signer](./remote-signer.md)
- [Light Client guides](./light-client.md)
  - [How to sync a light client](./light-client.md#)
- [Metrics](./metrics.md)
- [Logging](./logging.md)

## Node Types

We will cover the various types of node types within Tendermint.

### Full Node

 A full node is a node that participates in the network but will not help secure it. Full nodes can be used to store the entire state of a blockchain. For Tendermint there are two forms of state. First, blockchain state, this represents the blocks of a blockchain.  Secondly, there is Application state, this represents the state that transactions modify. The knowledge of how a transaction can modify state is not held by Tendermint but rather the application on the other side of the ABCI boundary.

 > Note: If you have not read about the seperation of consensus and application please take a few minutes to read up on it as it will provide a better understanding to many of the terms we use throughout the documentation. You can find more information on the ABCI [here](../app-dev/app-architecture.md).

 As a full node operator you are providing services to the network that helps it come to consensus and others catch up to the current block. Even though a full node only helps the network come to consensus it is important to secure your node from adversarial actors. We recommend using a firewall and a proxy if possible. Running a full node can be easy, but it varies from network to network. Verify your applications documentation prior running a node.

### Seed Nodes

 A seed node provides a node with a list of peers which a node can connect to. When starting a node you must provide at least one type of node to be able to connect to the desired network. By providing a seed node you will be able to populate your address quickly. A seed node will not be kept as a peer but will disconnect from your node after it has provided a list of peers.

### Sentry Node

 A sentry node is similar to a full node in almost every way. The difference is a sentry node will have one or more private peers. These peers may be validators or other full nodes in the network. A sentry node is meant to provide a layer of security for your validator, similar to how a firewall works with a computer.

### Validators

Validators are nodes that participate in the security of a network. Validators have an associated power in Tendermint, this power can represent stake in a [proof of stake](https://en.wikipedia.org/wiki/Proof_of_stake) system, reputation in [proof of authority](https://en.wikipedia.org/wiki/Proof_of_authority) or any sort of measurable unit. Running a secure and consistently online validator is crucial to a networks health. A validator must be secure and fault tolerant, it is recommended to run your validator with 2 or more sentry nodes.

As a validator there is the potential to have your weight reduced, this is defined by the application. Tendermint is notified by the application if a validator should have their weight increased or reduced. Application have different types of malicious behavior which lead to slashing of the validators power. Please check the documentation of the application you will be running in order to find more information.
