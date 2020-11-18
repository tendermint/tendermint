---
order: 1
parent:
  title: Nodes
  order: 4
---
# Overview

This section will focus on how to operate full nodes, validators and light clients.

- [Node Types](#node-types)
- [Configuration](./configuration.md)
        - State sync
- [Validator Guides](./validators.md)
  - [How to secure your keys](./validators.md#validator_keys)
  - How to start a sentry node architecture.
- [Light Client guides](./light-client.md)
  - [How to sync a light client](./light-client.md#)
- [Metrics](./metrics.md)

## Node Types

### Full Node

 A full node is a node that participates in the network but will not help secure it. Full nodes can be used to store the entire state of a blockchain. For Tendermint there are two forms of state. First, blockchain state, this represents the blocks of a blockchain.  Secondly, there is Application state, this represents the state that transactions modify. The knowledge of how a transaction can modify state is not held by Tendermint but rather the application on the other side of the ABCI boundary.

 > Note: If you have not read about the seperation of consensus and application please take a few minutes to read up on it as it will provide a better understanding to many of the terms we use throughout the documentation. You can find more information on the ABCI [here](../app-dev/app-architecture.md).

 As a full node operator you are providing services to the network that helps it come to consensus and others catch up to the current block. Even though a full node only helps the network come to consensus it is important to secure your node from adversarial actors. We recommend using a firewall and a proxy if possible. Operating a full node means as a user of the network you will be able to query and submit transactions through a trusted node. Running a full node can be easy, but it varies from network to network. Verify your applications documentation prior running a node.

### Seed Nodes

 A seed node provides a node with a list of peers which a node can connect to. When starting a node you must provide at least one type of node to be able to connect to the desired network.

 When a node starts it's address book will be empty, to populate an address book efficiently a node will connect to the seed node and request a list of peers for it to dial. If a node has seed mode enabled it will connect to peers provide them with a list of peers then disconnect.

### Sentry Node

 A sentry node is similar to a full node in almost every way. The difference is a sentry node will have one or more private peers. These peers may be validators or other full nodes in the network. A sentry node is meant to provide a layer of security for your validator, you can think of it as a firewall for the validator.

### Validators

Validators are nodes that participate in the security of a network. Running a secure and consistently online validator is crucial to networks A validator must be secure and fault tolerant, it is recommended to run your validator with 2 or more sentry nodes. A validator has an associated power in Tendermint, this power can represent stake ([proof of stake](https://en.wikipedia.org/wiki/Proof_of_stake)), reputation ([proof of authority](https://en.wikipedia.org/wiki/Proof_of_authority)) or various other things depending on the security model of the application. Applications have various

Depending on the application different types of malicious behavior can lead to slashing of the validators power. Please check the documentation of the application you will be running in order to find more information.
