---
order: 1
---

# Setup a Node

In this section we will walking through installing and configuring a Tendermint full node for our own network. Later in this section we will cover joining a network.

> Note: Instructions may vary based on the application.

There are a couple different types of nodes.

- **Full Node**: A full node is a node that participates in the network but will not help secure it. Full nodes are useful for users who would like to operate in order to query and submit transactions from a trusted source. Running a full node can be easy and consume a small amount of resources, but if you would like to keep the entire state of the blockchain a large hard drive will be needed. 
- **Seed Nodes**: A seed node is helpful for nodes that are starting up. When a node starts it's address book will be empty, to populate an address book efficiently a node will connect to the seed node and request a list of peers for it to dial. If a node has seed mode enabled it will connect to peers provide them with a list of peers they can connect to then disconnect. This type of node is useful for people operating countless nodes. 
- **Sentry Node**: A sentry node is similar to a full node in almost every way. The differences are that a sentry node will have one or many private peers. These peers may be validators or other full nodes in the network. Sentry nodes are recommended for securing validators, read more on how to setup a sentry network [here](./validators.md#setting-up-a-validator). 
- **Validators**: Validators are nodes that participate in the security of a network. A validator must be secure and fault tolerant. A validator has an associated power in Tendermint, this power can represent stake ([proof of stake](https://en.wikipedia.org/wiki/Proof_of_stake)), reputation ([proof of authority](https://en.wikipedia.org/wiki/Proof_of_authority)) or various other things depending on the security model of the application. Depending on the application different types of malicious behavior can lead to slashing of the validators power. Please check the documentation of the application you will be running in order to find more information. 

## Installation

There are two ways we can run a Tendermint node. 

1. By downloading the binary for our operating system [here](https://github.com/tendermint/tendermint/releases).
2. By installing go on our local machine and then compiling and installing the Tendermint Binary, you can find a walkthrough for this step [here](../introduction/install.md).

## Configuration

Tendermint has a configuration file with a plethora of settings that can be changed for specific use cases. For our walkthrough we will focus on:

- `external_address`: When `external_address` is populated the node will advertise this address. This is needed because currently the advertising of addresses does not work correctly. You can read more and follow the discussion in multiple issues (<https://github.com/tendermint/tendermint/issues/5588>,<https://github.com/tendermint/tendermint/issues/4260> and <https://github.com/tendermint/tendermint/issues/3716>)

- `laddr`: Laddr is the address that the RPC will be exposed on, we will be exposing it. It is defaulted to `127.0.0.1` and we would like to access it from a public connection. We recommend to not expose the RPC to the public without a proxy to handle connections. [Nginx](https://www.nginx.com/) is widely adopted as a tool to do the needed work of keeping your RPC connection secure and/or private.

- `persistent_peer`: Persistent Peer is a peer that you would like to be connected to at all times. This can help you bootstrap your node when joining a network. For our first node we will not have a persistent peer but for our second node we will insert the first nodes id and ip. 

- `seeds`: Seeds is a list of nodes that are seed nodes. If you are operating a node and dont have a persistent peer you would like to connect to, a seed node will be useful to help populate the nodes address book. 

These are only four of the many configurable parameters for Tendermint, to see more of the parameters and what they mean see [configuration](./configuration.md).

Applications may ask you to configure more parameters based on their needs.


## Joining a network

In a [previous section](../introduction/quick-start.md) we covered how to start a network. In this section we will cover how to join a network. 

There are a few steps that must be done before being able to join a network. 

- First, we must download the application and Tendermint. In the case of the [Cosmos-SDK](https://github.com/cosmos/cosmos-sdk/) Tendermint is built in so installing the application is all that's needed. 
- Secondly, we need to get the correct genesis. When initializing a node you will have seen that genesis was created as well. This genesis file does not represent network's you are trying to join genesis, we must go the applications documentation, find a link to the correct genesis or query a node's RPC endpoint `/genesis`. Once we have the genesis file we must replace the auto-generated genesis with the correct one. 
- Thirdly, we must get a peer or seed's node id and address. This is needed in order to boot strap our node with peers. 
