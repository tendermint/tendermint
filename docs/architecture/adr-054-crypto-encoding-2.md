# ADR 054: Crypto encoding (part 2)

## Changelog

\*2020-2-27: Created

## Context

Amino has been a pain point of many users in the ecosystem. While Tendermint does not suffer greatly from the performance degradation introduced by amino, we are making an effort in moving the encoding format to a widely adopt format, [Protocol Buffers](https://developers.google.com/protocol-buffers). With this migration a new standard is needed for the encoding of keys. This will cause ecosystem wide breaking changes.

Currently amino encodes keys as `<PrefixBytes> <Length> <ByteArray>`.

> With keys becoming a [oneof type](https://developers.google.com/protocol-buffers/docs/proto3#oneof) we have the option of removing the prefix bytes.

## Options

These are two options available, there may be others.

1. Remove backwards compatibility and go with `oneof` proto encoding of keys.

- This will be a major breaking change in the ecosystem.
- Will need a migration script to be released in conjunction with the release which will have this change.

> I am leaning towards the first option and go through the with headache in order to entirely remove the amino overhead

2. Keep the amino encoding format `<PrefixBytes> <Length> <ByteArray>` and protobuf encode the key after appending the prefix bytes.

- This will cause issues with bech32 as it will exceed the maximum length permitted by the libraries
- Backwards compatible, will not keep amino around but use custom marshlers in order to prefix the keys, less of a headache as no migration script would be needed and less coordination in the community.

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
