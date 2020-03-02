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

- Less headache, users will be happy for not needing to do a migration.

## Decision

Move all key types to protobuf encoding and have backwards compatibility available for where it is needed.

The places where backwards compatibility is needed is still unclear. I tried to break it down by usage of `cryptoamino.RegesiterAmino(cdc)` and looking through most of the reactors, we do not rely on a amino encoded key, and the key that is used in genesis and saved for use with validators is in json form, amino does not prefix bytes for json encoding so this will not be a problem.

Modules that use `crypto-amino`:

- State: This will not be a worry as this change will require a network upgrade (hardfork)
- Privval: The implementation of the privval server is reliant on amino encoded keys, but the key is not stored as amino encoded on disk. Therefore this issue will be resolved with a hardfork upgrade.
  - Organization with KMS and other tooling will need to be coordinated prior to release.
- P2P: There a few modules within the p2p package that define there own codec.
  - Conn: Secret Connection is a area that uses the amino encoded bytes of a key, this as well will not be a worry as there will be a hardfork upgrade and new secret connections will be established.
    <!-- I'm not sure how much the secret connection is actually used. -->
  - P2P: Here as well cryptoamino is used but backwards compatibility will not be needed as there will be a hardfork upgrade.
- Node: Here cryptoamino is used for unmarshaling & marshaling keys for genesis, since json encoding of amino does not prefix any bytes then this will be fine.
- lite: Within the lite package there is not a requirement for backwards compatibility of keys as there will be a hardfork upgrade as well.
- evidence: Evidence uses cryptoamino in order to unmarshal specific types of evidence which contain a key type. Again this will not need backwards compatibility as there is a need for a hardfork for these changes.

<!-- TODO: ask bez how he is currently working with encoded types. -->

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
