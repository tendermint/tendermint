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
  option (cosmos_proto.interface_type) = "*github.com/tendermint/tendermint/crypto.PubKey";
  oneof key {
    bytes ed25519 = 1
        [(gogoproto.casttype) = "github.com/tendermint/tendermint/crypto/ed25519.PubKey"];
    bytes secp256k1 = 2
        [(gogoproto.casttype) = "github.com/tendermint/tendermint/crypto/secp256k1.PubKey"];
    bytes sr25519 = 3
        [(gogoproto.casttype) = "github.com/tendermint/tendermint/crypto/sr25519.PubKey"];
    PubKeyMultiSigThreshold multisig = 4
        [(gogoproto.casttype) = "github.com/tendermint/tendermint/crypto/multisig.PubKeyMultisigThreshold"];;
  }

message PrivKey {
  option (cosmos_proto.interface_type) = "github.com/tendermint/tendermint/crypto.PrivKey";
  oneof sum {
    bytes ed25519 = 1
        [(gogoproto.casttype) = "github.com/tendermint/tendermint/crypto/ed25519.PrivKey"];
    bytes secp256k1 = 2
        [(gogoproto.casttype) = "github.com/tendermint/tendermint/crypto/secp256k1.PrivKey"];
    bytes sr25519 = 3
        [(gogoproto.casttype) = "github.com/tendermint/tendermint/crypto/sr25519.PrivKey"];;
  }
}
```

> Note: The places where backwards compatibility is needed is still unclear.

All modules currently do not rely on amino encoded bytes and keys are not amino encoded for genesis, therefore a hardfork upgrade is what will be needed to adopt these changes.

This work will be broken out into a few PRs, this work will be merged into a proto-breakage branch, all PRs will be reviewed prior to being merged:

1. Encoding of keys to protobuf and protobuf messages
2. Move Tendermint types to protobuf, mainly the ones that are being encoded.
3. Go one by one through the reactors and transition amino encoded messages to protobuf.
4. Test with cosmos-sdk and/or testnets repo.

## Status

Proposed

## Consequences

- Move keys to protobuf encoding, where backwards compatibility is needed, amino marshal and unmarshal functions will be used.

### Positive

- Protocol Buffer encoding will not change going forward.
- Removing amino overhead from keys will help with the KSM.
- Have a large ecosystem of supported languages.

### Negative

- Hardfork is required to integrate this into running chains.

### Neutral

## References

> Are there any relevant PR comments, issues that led up to this, or articles referenced for why we made the given design choice? If so link them here!

- {reference link}
