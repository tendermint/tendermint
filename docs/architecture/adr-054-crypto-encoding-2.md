# ADR 054: Crypto encoding (part 2)

## Changelog

\*2020-2-27: Created

## Context

Amino has been a pain point of many users in the ecosystem. While Tendermint does not suffer greatly from the performance degradation introduced by amino, we are making an effort in moving the encoding format to a widely adopted format, [Protocol Buffers](https://developers.google.com/protocol-buffers). With this migration a new standard is needed for the encoding of keys. This will cause ecosystem wide breaking changes.

Currently amino encodes keys as `<PrefixBytes> <Length> <ByteArray>`.

> With keys becoming a [oneof type](https://developers.google.com/protocol-buffers/docs/proto3#oneof) we have the option of removing the prefix bytes.

## Decision

Transitioning from a fixed size byte array to bytes would be the first step. This will enable usage of [cosmos-proto](https://github.com/regen-network/cosmos-proto) interface type. This removes boiler plate needed for oneof types.

The approach that will be taken to minimize headaches for users is one where all encoding of keys will shift to protobuf and where amino encoding is relied on, there will be custom marshal and unmarshal functions.

Protobuf messages:

```proto
message PubKey {
  option (cosmos_proto.interface_type) = "github.com/tendermint/tendermint/crypto.PubKey";
  oneof sum {
    bytes                   ed25519   = 1;
    bytes                   secp256k1 = 2;
    bytes                   sr25519   = 3;
    PubKeyMultiSigThreshold multisig  = 4;
  }
}

message PrivKey {
  option (cosmos_proto.interface_type) = "github.com/tendermint/tendermint/crypto.PrivKey";
  oneof sum {
    bytes                   ed25519   = 1;
    bytes                   secp256k1 = 2;
    bytes                   sr25519   = 3;
  }
}
```

> Note: The places where backwards compatibility is needed is still unclear. Still need to dive into the code a bit.

All modules currently do not rely on amino encoded bytes and keys are not amino encoded for genesis, therefore a hardfork upgrade is what will be needed to adopt these changes.

<!-- TODO: define the above better, need to read the code a bit more -->

This work will be broken out into a few PRs. One for the encoding of keys to protobuf and then other PRs will have types and reactor changes.

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
