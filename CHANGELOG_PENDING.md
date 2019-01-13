## v0.28.0

*January 14th, 2019*

Special thanks to external contributors on this release:
@fmauricios, @gianfelipe93, @husio, @needkane, @srmo, @yutianwu

This release is primarily about upgrades to the `privval` system -
separating the `priv_validator.json` into distinct config and data files, and
refactoring the socket validator to support reconnections.

See [UPGRADING.md](UPGRADING.md) for more details.

### BREAKING CHANGES:

* CLI/RPC/Config
- [cli] Removed `node` `--proxy_app=dummy` option. Use `kvstore` (`persistent_kvstore`) instead.
- [cli] Renamed `node` `--proxy_app=nilapp` to `--proxy_app=noop`.
- [config] [\#2992](https://github.com/tendermint/tendermint/issues/2992) `allow_duplicate_ip` is now set to false
- [privval] [\#1181](https://github.com/tendermint/tendermint/issues/1181) Split immutable and mutable parts of `priv_validator.json`
  (@yutianwu)
- [privval] [\#2926](https://github.com/tendermint/tendermint/issues/2926) Split up `PubKeyMsg` into `PubKeyRequest` and `PubKeyResponse` to be consistent with other message types
- [privval] [\#2923](https://github.com/tendermint/tendermint/issues/2923) Listen for unix socket connections instead of dialing them

* Apps

* Go API
- [types] [\#2981](https://github.com/tendermint/tendermint/issues/2981) Remove `PrivValidator.GetAddress()`

* Blockchain Protocol

* P2P Protocol

### FEATURES:
- [rpc] [\#3052](https://github.com/tendermint/tendermint/issues/3052) Include peer's remote IP in `/net_info`

### IMPROVEMENTS:
- [consensus] [\#3086](https://github.com/tendermint/tendermint/issues/3086) Log peerID on ignored votes (@srmo)
- [docs] [\#3061](https://github.com/tendermint/tendermint/issues/3061) Added spec on signing consensus msgs at
  ./docs/spec/consensus/signing.md
- [privval] [\#2948](https://github.com/tendermint/tendermint/issues/2948) Memoize pubkey so it's only requested once on startup
- [privval] [\#2923](https://github.com/tendermint/tendermint/issues/2923) Retry RemoteSigner connections on error

### BUG FIXES:

- [types] [\#2926](https://github.com/tendermint/tendermint/issues/2926) Do not panic if retrieving the private validator's public key fails
- [rpc] [\#3053](https://github.com/tendermint/tendermint/issues/3053) Fix internal error in `/tx_search` when results are empty
  (@gianfelipe93)
- [crypto/multisig] [\#3102](https://github.com/tendermint/tendermint/issues/3102) Fix multisig keys address length
- [crypto/encoding] [\#3101](https://github.com/tendermint/tendermint/issues/3101) Fix `PubKeyMultisigThreshold` unmarshalling into `crypto.PubKey` interface
- [build] [\#3085](https://github.com/tendermint/tendermint/issues/3085) Fix `Version` field in build scripts (@husio)
- [p2p/conn] [\#3111](https://github.com/tendermint/tendermint/issues/3111) Make SecretConnection thread safe
