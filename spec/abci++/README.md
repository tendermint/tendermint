---
order: 1
parent:
  title: ABCI++
  order: 3
---

# ABCI++

## Introduction

ABCI++ is a major evolution of ABCI (**A**pplication **B**lock**c**hain **I**nterface).
Like its predecessor, ABCI++ is the interface between Tendermint (a state-machine
replication engine) and the actual state machine being replicated (i.e., the Application).
The API consists of a set of _methods_, each with a corresponding `Request` and `Response`
message type.

The methods are always initiated by Tendermint. The Application implements its logic
for handling all ABCI++ methods.
Thus, Tendermint always sends the `Request*` messages and receives the `Response*` messages
in return.

All ABCI++ messages and methods are defined in
[protocol buffers](https://github.com/tendermint/tendermint/blob/master/proto/spec/abci/types.proto).
This allows Tendermint to run with applications written in many programming languages.

This specification is split as follows:

- [Basic concepts and definitions](./abci++_basic_concepts_002_draft.md) - definitions and descriptions
  of concepts that are needed to understand other parts of this sepcification.
- [Methods](./abci++_methods_002_draft.md) - complete details on all ABCI++ methods
  and message types.
- [Requirements for the Application](./abci++_app_requirements_002_draft.md) - formal requirements
  on the Application's logic to ensure liveness of Tendermint. These requirements define what
  Tendermint expects from the Application.
- [Tendermint's expected behavior](./abci++_tmint_expected_behavior_002_draft.md) - specification of
  how the different ABCI++ methods may be called by Tendermint. This explains what the Application
  is to expect from Tendermint.

>**TODO** Re-read these and remove redundant info

- [Applications](../abci/apps.md) - how to manage ABCI application state and other
  details about building ABCI applications
- [Client and Server](../abci/client-server.md) - for those looking to implement their
  own ABCI application servers
