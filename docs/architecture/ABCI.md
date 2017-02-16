# ABCI

ABCI is an interface between the consensus/blockchain engine known as tendermint, and the application-specific business logic, known as an ABCi app.

The tendermint core should run unchanged for all apps.  Each app can customize it, the supported transactions, queries, even the validator sets and how to handle staking / slashing stake. This customization is achieved by implementing the ABCi app to send the proper information to the tendermint engine to perform as directed.

To understand this decision better, think of the design of the tendermint engine.

* A blockchain is simply consensus on a unique global ordering of events.
* This consensus can efficiently be implemented using BFT and PoS
* This code can be generalized to easily support a large number of blockchains
* The block-chain specific code, the interpretation of the individual events, can be implemented by a 3rd party app without touching the consensus engine core
* Use an efficient, language-agnostic layer to implement this (ABCi)


Bucky, please make this doc real.
