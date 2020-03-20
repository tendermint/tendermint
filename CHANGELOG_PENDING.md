## v0.33.3

\*\*

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

- CLI/RPC/Config

- [types] \#4382 `BlockIdFlag` and `SignedMsgType` has moved to a protobuf enum types
- [types] \#4382 `PartSetHeader` has become a protobuf type, `Total` has been changed from a `int` to a `int32`
- [types] \#4382  `BlockID` has become a protobuf type
- [types] \#4382  enum `CheckTxType` values have been made uppercase: `NEW` & `RECHECK`
- [crypto] \#4460 Introduce `PubKey` & `PrivKey` proto types only for tendermint use.
- [crypto] \#4460 `Ed25519`, `Secp256k1` & `Sr25519` are now []byte
- [crypto] \#4460 `Byte()` method on all keys now return the raw bytes of the key
- [crypto] \#4460 Remove suffixes from all keys. 
    - ed25519: type `PrivKeyEd25519` is now `PrivKey`
    - ed25519: type `PubKeyEd25519` is now `PubKey`
    - secp256k1: type`PrivKeySecp256k1` is now `PrivKey`
    - secp256k1: type`PubKeySecp256k1` is now `PubKey`
    - sr25519: type `PrivKeySr25519` is now `PrivKey`
    - sr25519: type `PubKeySr25519` is now `PubKey`


- Apps

- Go API

### FEATURES:

### IMPROVEMENTS:

- [p2p] [\#4548](https://github.com/tendermint/tendermint/pull/4548) Add ban list to address book (@cmwaters)
- [privval] \#4534 Add `error` as a return value on`GetPubKey()`

### BUG FIXES:
