---
order: 1
parent:
  title: ABCI
  order: 2
---

# ABCI

ABCI stands for "**A**pplication **B**lock**c**hain **I**nterface".
ABCI is the interface between Tendermint (a state-machine replication engine)
and your application (the actual state machine). It consists of a set of
_methods_, each with a corresponding `Request` and `Response`message type. 
To perform state-machine replication, Tendermint calls the ABCI methods on the 
ABCI application by sending the `Request*` messages and receiving the `Response*` messages in return.

All ABCI messages and methods are defined in [protocol buffers](https://github.com/tendermint/tendermint/blob/v0.34.x/proto/abci/types.proto). 
This allows Tendermint to run with applications written in many programming languages.

This specification is split as follows:

- [Methods and Types](./abci.md) - complete details on all ABCI methods and
  message types
- [Applications](./apps.md) - how to manage ABCI application state and other
  details about building ABCI applications
- [Client and Server](./client-server.md) - for those looking to implement their
  own ABCI application servers
