# RFC 008 : Deterministic Proto Byte Serialization

## Changelog

- 09-Dec-2021: Initial draft (@williambanfield).

## Abstract

This document discusses the issue of stable byte-representation of serialized messages
within Tendermint and describes a few possible routes that could be taken to address it.

## Background

We use the byte representations of wire-format proto messages to produce
and verify hashes of data within the Tendermint codebase as well as for
producing and verifying cryptographic signatures over these signed bytes.

The protocol buffer encoding spec does not guarantee that the byte representation
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
2. Encoding order of elements of a repeated field.
3. Presence or absence of default values.
4. Inclusion/exclusion of unknown fields.
5. Padded length of varint fields.

We have a few options to consider when producing this stable representation.

### Use only compliant serializers and constrain field usage

According to [Cosmos-SDK ADR-27](cosmos-sdk-adr-27), when message types obey a simple 
set of rules, gogoproto produces a consistent byte representation of serialized messages.
This seems promising, although more research is needed to guarantee gogoproto always
produces a consistent set of bytes on serialized messages. This would solve the problem 
within Tendermint as written in Go, but would require ensuring that there are 

### Reorder serialized bytes to ensure determinism.

The serialized form of a proto message can be transformed into a canonical representation
by applying simple rules to the serialized bytes. Re-ordering the serialized bytes
would allow Tendermint to produce a canonical byte representation without having to 
simultaneously maintain a custom proto marshaller. 

This could be implemented as a function in many languages that performed the following steps:

1. Reordered all fields to be in tag-sorted order.
2. Reordered all `repeated` sub-fields to be in lexicographically sorted order.
3. Deleted all default values from the representation.
5. Trimmed all varints to be of minimum length.

This would still require that messages never unmarshal data structures with unknown fields.
This can be accomplished by defining adding fields to a structure that needs canonicalization
as a breaking changes within our semantic versioning and disallowing it within a major version.

Finally, we should add clear documentation to the Tendermint codebase every time we
compare hashes of proto messages or use proto serialized bytes to produces a
digital signatures that we have been careful to ensure that the hashes are performed
properly.

### References

[proto-spec]()
[spec-issue]()
[cosmos-sdk-adr-27]()
[cer-proto-3](https://github.com/regen-network/canonical-proto3)

