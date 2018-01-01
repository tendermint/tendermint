# Application BlockChain Interface (ABCI)

[![CircleCI](https://circleci.com/gh/tendermint/abci.svg?style=svg)](https://circleci.com/gh/tendermint/abci)

Blockchains are systems for multi-master state machine replication.
**ABCI** is an interface that defines the boundary between the replication engine (the blockchain),
and the state machine (the application).
By using a socket protocol, we enable a consensus engine running in one process
to manage an application state running in another.

For background information on ABCI, motivations, and tendermint, please visit [the documentation](http://tendermint.readthedocs.io/en/master/).
The two guides to focus on are the `Application Development Guide` and `Using ABCI-CLI`.

Previously, the ABCI was referred to as TMSP.

The community has provided a number of addtional implementations, see the [Tendermint Ecosystem](https://tendermint.com/ecosystem)

## Implementation

We provide three implementations of the ABCI in Go:

- Golang in-process
- ABCI-socket
- GRPC

Note the GRPC version is maintained primarily to simplify onboarding and prototyping and is not receiving the same
attention to security and performance as the others.

### In Process

The simplest implementation just uses function calls within Go.
This means ABCI applications written in Golang can be compiled with TendermintCore and run as a single binary.

### Socket (TSP)

ABCI is best implemented as a streaming protocol.
The socket implementation provides for asynchronous, ordered message passing over unix or tcp.
Messages are serialized using Protobuf3 and length-prefixed.
Protobuf3 doesn't have an official length-prefix standard, so we use our own.  The first byte represents the length of the big-endian encoded length.

For example, if the Protobuf3 encoded ABCI message is `0xDEADBEEF` (4 bytes), the length-prefixed message is `0x0104DEADBEEF`.  If the Protobuf3 encoded ABCI message is 65535 bytes long, the length-prefixed message would be like `0x02FFFF...`.

### GRPC

GRPC is an rpc framework native to Protocol Buffers with support in many languages.
Implementing the ABCI using GRPC can allow for faster prototyping, but is expected to be much slower than
the ordered, asynchronous socket protocol. The implementation has also not received as much testing or review.

Note the length-prefixing used in the socket implementation does not apply for GRPC.

## Usage

The `abci-cli` tool wraps an ABCI client and can be used for probing/testing an ABCI server.
For instance, `abci-cli test` will run a test sequence against a listening server running the Counter application (see below).
It can also be used to run some example applications.
See [the documentation](http://tendermint.readthedocs.io/en/master/) for more details.

### Example Apps

Multiple example apps are included:
- the `abci-cli counter` application, which illustrates nonce checking in txs
- the `abci-cli dummy` application, which illustrates a simple key-value Merkle tree
- the `abci-cli dummy --persistent` application, which augments the dummy with persistence and validator set changes

### Install

```
go get github.com/tendermint/abci
cd $GOPATH/src/github.com/tendermint/abci
make get_vendor_deps
make install
```

## Specification

See the [spec file](specification.rst) for more information.
