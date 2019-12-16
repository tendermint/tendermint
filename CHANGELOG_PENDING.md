## v0.33.1

\*\*

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty
program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

- CLI/RPC/Config

- Apps

- Go API

### FEATURES:

### IMPROVEMENTS:

### BUG FIXES:

- [rpc] [#\4319] Check BlockMeta is not nil in Blocks & BlockByHash
- [rpc] \#3188 Added `block_size` to `BlockMeta` this is reflected in `/blockchain`
- [types] \#2521 Add `NumTxs` to `BlockMeta` and `EventDataNewBlockHeader`
- [docs] [\#4111](https://github.com/tendermint/tendermint/issues/4111) Replaced dead whitepaper link in README.md
- [p2p] [\#4185](https://github.com/tendermint/tendermint/pull/4185) Simplify `SecretConnection` handshake with merlin
- [cli] [\#4065](https://github.com/tendermint/tendermint/issues/4065) Add `--consensus.create_empty_blocks_interval` flag (@jgimeno)
- [docs] [\#4065](https://github.com/tendermint/tendermint/issues/4065) Document `--consensus.create_empty_blocks_interval` flag (@jgimeno)
- [crypto] [\#4190](https://github.com/tendermint/tendermint/pull/4190) Added SR25519 signature scheme
- [abci] [\#4177] kvstore: Return `LastBlockHeight` and `LastBlockAppHash` in `Info` (@princesinha19)
- [rpc] [\#2741](https://github.com/tendermint/tendermint/issues/2741) Add `proposer` to `/consensus_state` response (@princesinha19)
- [libs] [\#2956](https://github.com/tendermint/tendermint/issues/2956) rands: Removed `mutex` (@pushcodeveryday)


