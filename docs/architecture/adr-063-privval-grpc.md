# ADR 063: Privval gRPC

## Changelog

- 23/11/2020: Initial Version (@marbar3778)

## Context

Validators use remote signers to help secure their keys. This system is Tendermint's recommended way to secure validators, but the path to integration with Tendermint's private validator client is plagued with custom protocols. 

Tendermint uses its own custom secure connection protocol (`SecretConnection`) and a raw tcp connection protocol. The secure connection protocol until recently was exposed to man in the middle attacks and limits the amount and speed of integrations users can make. The raw tcp connection protocol is less custom, but has been causing minuet issues with users. 

Migrating Tendermint's private validator to a widely adopted protocol will ease the current maintenance and integration burden experienced with the current protocol. 

## Decision

After discussing with multiple stake holders [gRPC](https://grpc.io/) was decided on to replace the current private validator protocol. GRPC is a widely adopted protocol in the micro-service and cloud infrastructure world. GRPC uses [protocol-buffers](https://developers.google.com/protocol-buffers) to describe its service definition providing a language agnostic implementation. Tendermint uses protobuf for on disk and over the wire encoding. 

## Alternative Approaches

- JSON-RPC: We did not consider JSON-RPC because Tendermint uses protobuf extensively make gRPC a natural choice. JSON-RPC passes information of the wire in JSON. This can lead to performance bottle necks for low block time chains. 

## Detailed Design

With the recent integration of [Protobuf](https://developers.google.com/protocol-buffers) into Tendermint the needed changes to migrate from the current private validator protocol to gRPC is not large. 

The [service definition](https://grpc.io/docs/what-is-grpc/core-concepts/#service-definition) for gRPC will be defined as:

```proto
service PrivValidatorAPI {
  rpc GetPubKey(tendermint.proto.privval.PubKeyRequest) returns (tendermint.proto.privval.PubKeyResponse);
  rpc SignVote(tendermint.proto.privval.SignVoteRequest) returns (tendermint.proto.privval.SignedVoteResponse);
  rpc SignProposal(tendermint.proto.privval.SignProposalRequest) returns (tendermint.proto.privval.SignedProposalResponse);
}
```

#### Keep Alive

If you have worked on the private validator system you will see that we are removing the `PingRequest` and `PingResponse` messages. These messages were used to create functionality which kept the connection alive. With gRPC there is a [keep alive feature](https://github.com/grpc/grpc/blob/master/doc/keepalive.md) that will be added along side the integration to provide the same functionality. 

#### Metrics

Remote signers are crucial to operating secure and consistently up Validators. In the past there were no metrics to tell the operator if something is wrong other than the node not signing. Integrating metrics into the client and provided server will be done with [prometheus](https://github.com/grpc-ecosystem/go-grpc-prometheus). This will be integrated into node's prometheus export for node operators. 

#### Security

[TLS](https://en.wikipedia.org/wiki/Transport_Layer_Security) is widely adopted with the use of gRPC. The private validator server and client will require TLS files in order to run. This requires users to generate both client and server certificates for a TLS connection. Tendermint will provide a make command to help users with the generation of this.  A --insecure flag will be provided for development/testing purposes. 

#### Upgrade Path

This is a largely breaking change for validator operators. The optimal upgrade path would be to release gRPC in a minor release, allow kms to use the upgraded system, then in the next major release the current system is removed. This allows users to migrate to the new system and not have to coordinate upgrading the key management system alongside a network upgrade. 

The upgrade of tmkms will be coordinated with Iqlusion. They will be able to make the necessary upgrades to allow users to migrate to gRPC from the current protocol. 

## Status

Proposed

## Consequences

> This section describes the consequences, after applying the decision. All consequences should be summarized here, not just the "positive" ones.

### Positive

- Use an adopted standard for secure communication. (TLS)
- Use an adopted communication protocol. (gRPC)
- Requests are multiplexed onto the tcp connection. (http/2)
- Language agnostic service definition.

### Negative

- Use of http adds an overhead to the TCP connection.
- Users will need to generate certificates to use TLS. (Added step)

### Neutral

## References

> Are there any relevant PR comments, issues that led up to this, or articles referenced for why we made the given design choice? If so link them here!

- {reference link}
