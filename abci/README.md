# Application BlockChain Interface (ABCI)

Blockchains are systems for multi-master state machine replication.
**ABCI** is an interface that defines the boundary between the replication engine (the blockchain),
and the state machine (the application).
Using a socket protocol, a consensus engine running in one process
can manage an application state running in another.

Previously, the ABCI was referred to as TMSP.

The community has provided a number of additional implementations, see the [Tendermint Ecosystem](https://github.com/tendermint/awesome#ecosystem)


## Installation & Usage

To get up and running quickly, see the [getting started guide](../docs/app-dev/getting-started.md) along with the [abci-cli documentation](../docs/app-dev/abci-cli.md) which will go through the examples found in the [examples](./example/) directory.

## Specification

A detailed description of the ABCI methods and message types is contained in:

- [The main spec](https://github.com/tendermint/spec/blob/master/spec/abci/abci.md)
- [A protobuf file](./types/types.proto)
- [A Go interface](./types/application.go)

## Protocol Buffers

To compile the protobuf file, run (from the root of the repo):

```sh
make protoc_abci
```

See `protoc --help` and [the Protocol Buffers site](https://developers.google.com/protocol-buffers)
for details on compiling for other languages. Note we also include a [GRPC](https://www.grpc.io/docs)
service definition.

## Evidence

A part of Tendermint's security model is the use of evidence which serves as proof of
malicious behaviour by a network participant. It is the responsibility of Tendermint
to detect such malicious behaviour, to gossip this and commit it to the chain and once
verified by all validators to pass it on to the application through the ABCI. It is the
responsibility of the application then to handle the evidence and exercise punishment.

Evidence has the following protobuf format:

```proto
enum EvidenceType {
  UNKNOWN               = 0;
  DUPLICATE_VOTE        = 1;
  LIGHT_CLIENT_ATTACK   = 2;
}

message Evidence {
  EvidenceType              type      = 1;
  // The offending validator
  Validator                 validator = 2 [(gogoproto.nullable) = false];
  // The height when the offense occurred
  int64                     height    = 3;
  // The corresponding time where the offense occurred
  google.protobuf.Timestamp time      = 4 [
    (gogoproto.nullable) = false,
    (gogoproto.stdtime)  = true
  ];
  // Total voting power of the validator set in case the ABCI application does
  // not store historical validators.
  // https://github.com/tendermint/tendermint/issues/4581
  int64 total_voting_power = 5;
}
```

There are two forms of evidence: Duplicate Vote and Light Client Attack. More
information can be found [here](https://github.com/tendermint/spec/blob/master/spec/light-client/accountability.md)
