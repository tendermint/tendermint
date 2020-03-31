# ADR 055: Protobuf Design

## Changelog

- 20202-3-31: Created

## Context

Currently we use [go-amino](https://github.com/tendermint/go-amino) throughout Tendermint. Amino enables quick prototyping and development of features. While this is nice amino does not provide the performance and developer convenience that is expected. For this reason moving to a different encoding format that is widely adopted and supports multiple languages is needed.

There are a few options to pick from:

- `Protobuf`: Protocol buffers are Google's language-neutral, platform-neutral, extensible mechanism for serializing structured data – think XML, but smaller, faster, and simpler. It is supported in countless languages and has been proven in production for many years.

- `FlatBuffers`: FlatBuffers is an efficient cross platform serialization library. Flatbuffers are more efficient than Protobuf due to the fast that there is no parsing/unpacking to a second representation. FlatBuffers has been tested and used in production but is not widely adopted.

- `CapnProto`: Cap’n Proto is an insanely fast data interchange format and capability-based RPC system. Cap'n Proto does not have a encoding/decoding step. It has not seen wide adoption throughout the industry.

## Decision

Transition Tendermint to Protobuf because of its performance and tooling. The Ecosystem behind Protobuf is vast and has outstanding [support for many languages](https://developers.google.com/protocol-buffers/docs/tutorials).

To make this move possible we will not be removing handwritten types but instead creating a `/proto` directory in which all the `.proto` files and types will live. Then where encoding is needed, on disk and over the wire, we will call util functions that will transition the types from handwritten go types to protobuf generated types.

By going with this design we will enable future changes to types and allow for a more modular codebase.

## Status

Proposed

## Consequences

### Positive

- Allows for modular types in the future
- Less refactoring
- Allows the proto files to be pulled into the spec repo in the future.

### Negative

- When a developer is updating a type they need to make sure to update the proto type as well

### Neutral

## References
