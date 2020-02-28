# ADR 054: Crypto encoding (part 2)

## Changelog

\*2020-2-27: Created

## Context

Amino has been a pain point of many users in the ecosystem. While Tendermint does not suffer greatly from the performance degradation introduced by amino, we are making an effort in moving the encoding format to a widely adopted format, [Protocol Buffers](https://developers.google.com/protocol-buffers). With this migration a new standard is needed for the encoding of keys. This will cause ecosystem wide breaking changes.

Currently amino encodes keys as `<PrefixBytes> <Length> <ByteArray>`.

> With keys becoming a [oneof type](https://developers.google.com/protocol-buffers/docs/proto3#oneof) we have the option of removing the prefix bytes.

## Options

If there are more options please leave a comment and we can discuss it.

1. Remove backwards compatibility and go with `oneof` proto encoding of keys.

- This will be a major breaking change in the ecosystem.
- Will need a migration script to be released in conjunction with the release which will have this change.

2. Use proto encoding for over the wire communication. Where backwards compatibility is needed use the amino encoding format: `<PrefixBytes> <Length> <ByteArray>`.

- This is a good proposal but would be a bit hacky. 
- Less headache, users will be happy for not needing to do a migration.

## Decision

> This section explains all of the details of the proposed solution, including implementation details.
> It should also describe affects / corollary items that may need to be changed as a part of this.
> If the proposed change will be large, please also indicate a way to do the change to maximize ease of review.
> (e.g. the optimal split of things to do between separate PR's)

## Status

Proposed

## Consequences

- Depends on which option is chosen

### Positive

- Protocol Buffer encoding will not change going forward.
- Removing amino overhead from keys will help with the KSM.
- Have a large ecosystem of supported languages.

### Negative

-

### Neutral

## References

> Are there any relevant PR comments, issues that led up to this, or articles referenced for why we made the given design choice? If so link them here!

- {reference link}
