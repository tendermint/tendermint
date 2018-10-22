# Overview

ABCI is the interface between Tendermint (a state-machine replication engine)
and your application (the actual state machine). It consists of a set of
*methods*, where each method has a corresponding `Request` and `Response`
message type. Tendermint calls the ABCI methods on the ABCI application by sending the `Request*`
messages and receiving the `Response*` messages in return.

All message types are defined in a [protobuf file](https://github.com/tendermint/tendermint/blob/develop/abci/types/types.proto).
This allows Tendermint to run applications written in any programming language.

This specification is split as follows:

- [Methods and Types](./abci.md) - complete details on all ABCI methods and
  message types
- [Applications](./apps.md) - how to manage ABCI application state and other
  details about building ABCI applications
- [Client and Server](./client-server.md) - for those looking to implement their
  own ABCI application servers
