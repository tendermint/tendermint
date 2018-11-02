# ABCI Implementations

We provide three implementations of the ABCI in written in Go:

- In-process (Golang)
- ABCI-socket
- GRPC

Note the GRPC version is maintained primarily to simplify onboarding and prototyping and is not receiving the same
attention to security and performance as the others

## In Process

The simplest implementation just uses function calls within Go.
This means ABCI applications written in Golang can be compiled with TendermintCore and run as a single binary.

## Socket (TSP)

ABCI is best implemented as a streaming protocol.
The socket implementation provides for asynchronous, ordered message passing over unix or tcp.
Messages are serialized using Protobuf3 and length-prefixed with a [signed Varint](https://developers.google.com/protocol-buffers/docs/encoding?csw=1#signed-integers)

For example, if the Protobuf3 encoded ABCI message is `0xDEADBEEF` (4 bytes), the length-prefixed message is `0x08DEADBEEF`, since `0x08` is the signed varint
encoding of `4`. If the Protobuf3 encoded ABCI message is 65535 bytes long, the length-prefixed message would be like `0xFEFF07...`.

Note the benefit of using this `varint` encoding over the old version (where integers were encoded as `<len of len><big endian len>` is that
it is the standard way to encode integers in Protobuf. It is also generally shorter.

## GRPC

GRPC is an rpc framework native to Protocol Buffers with support in many languages.
Implementing the ABCI using GRPC can allow for faster prototyping, but is expected to be much slower than
the ordered, asynchronous socket protocol. The implementation has also not received as much testing or review.

Note the length-prefixing used in the socket implementation does not apply for GRPC.
