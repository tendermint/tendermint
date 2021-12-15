# RFC 007 : Deterministic Proto Byte Serialization

## Changelog

- 09-Dec-2021: Initial draft (@williambanfield).

## Abstract

This document discusses the issue of stable byte-representation of serialized messages
within Tendermint and describes a few possible routes that could be taken to address it.

## Background

We use the byte representations of wire-format proto messages to produce
and verify hashes of data within the Tendermint codebase as well as for
producing and verifying cryptographic signatures over these signed bytes.

The protocol buffer [encoding spec][proto-spec-encoding] does not guarantee that the byte representation
of a protocol buffer message will be the same between two calls to an encoder.
While there is a mode to force the encoder to produce the same byte representation
of messages within a single binary, these guarantees are not good enough for our
use case in Tendermint. We require multiple different versions of a binary running
Tendermint to be able to inter-operate. Additionally, we require that multiple different
systems written in _different languages_ be able to participate in different aspects
of the protocols of Tendermint and be able to verify the integrity of the messages
they each produce.

While this has not yet created a problem that we know of in a running network, we should
make sure to provide stronger guarantees around the serialized representation of the messages
used within the Tendermint consensus algorithm to prevent any issue from occurring.


## Discussion

Proto has the following points of variability that can produce non-deterministic byte representation:

1. Encoding order of fields within a message.

Proto allows fields to be encoded in any order and even be repeated.

2. Encoding order of elements of a repeated field.

`repeated` fields in a proto message can be serialized in any order.

3. Presence or absence of default values.

Types in proto have defined default values similar to Go's zero values. 
Writing or omitting a default value are both legal ways of encoding a wire message.

4. Serialization of 'unknown' fields. 

Unknown fields can be present when a message is created by a binary with a newer 
version of the proto that contains fields that the deserializer in a different 
binary does not yet know about. Deserializers in binaries that do not know about the field
will maintain the bytes of the unknown field but not place them into the deserialized structure.

We have a few options to consider when producing this stable representation.

### Use only compliant serializers and constrain field usage

According to [Cosmos-SDK ADR-27][cosmos-sdk-adr-27], when message types obey a simple 
set of rules, gogoproto produces a consistent byte representation of serialized messages.
This seems promising, although more research is needed to guarantee gogoproto always
produces a consistent set of bytes on serialized messages. This would solve the problem 
within Tendermint as written in Go, but would require ensuring that there are similar
serializers written in other languages that produce the same output as gogoproto.

### Reorder serialized bytes to ensure determinism.

The serialized form of a proto message can be transformed into a canonical representation
by applying simple rules to the serialized bytes. Re-ordering the serialized bytes
would allow Tendermint to produce a canonical byte representation without having to
simultaneously maintain a custom proto marshaller.

This could be implemented as a function in many languages that performed the following when hashing:

1. Reordered all fields to be in tag-sorted order.
2. Does not add any of the data from unknown fields into the type to hash.
3. Reordered all non-scalar `repeated` sub-fields to be in lexicographically sorted order.
4. Deleted all default values from the byte representation.

Tendermint should not run into a case where it needs to verify the integrity of 
data with unknown fields for the following reasons:

When a process is checking hash equality, it knows the concrete data that it is trying to
compare to the hash. The purpose of checking hash equality within Tendermint is to ensure that
the data that process knows about matches the data that the network agreed on. There should
therefore not be a case where a process is checking hash equality using data that it did not expect
to receive. What the data represent may be opaque to the process, such as when checking the
hashes of transactions in a block, _but the validator still expected to receive this data_,
despite not understanding what their internal structure is. 

The same reasoning applies for signature verification within Tendermint. Processes
verify that a digital signature signed over a set of bytes by locally reconstructing the 
data structure that the digital signature signed.

A prototype implementation by @creachadair of this can be found in [the wirepb repo][wire-pb].
This could be implemented in multiple languages more simply than ensuring that there are
canonical proto serializers that match in each language.

Finally, we should add clear documentation to the Tendermint codebase every time we
compare hashes of proto messages or use proto serialized bytes to produces a
digital signatures that we have been careful to ensure that the hashes are performed
properly.

### References

[proto-spec-encoding]: https://developers.google.com/protocol-buffers/docs/encoding
[spec-issue]: https://github.com/tendermint/tendermint/issues/5005
[cosmos-sdk-adr-27]: https://github.com/cosmos/cosmos-sdk/blob/master/docs/architecture/adr-027-deterministic-protobuf-serialization.md
[cer-proto-3]: https://github.com/regen-network/canonical-proto3
[wire-pb]: https://github.com/creachadair/wirepb

