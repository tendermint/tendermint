## v0.32.9

\*\*

This release contains breaking changes to the `Block#Header`, specifically
`NumTxs` and `TotalTxs` were removed (\#2521). Here's how this change affects
different modules:

- apps: it breaks the ABCI header field numbering
- state: it breaks the format of `State` on disk
- RPC: all RPC requests which expose the header broke
- Go API: the `Header` broke
- P2P: since blocks go over the wire, technically the P2P protocol broke

Also, blocks are significantly smaller 🔥 because we got rid of the redundant
information in `Block#LastCommit`. `Commit` now mainly consists of a signature
and a validator address plus a timestamp. Note we may remove the validator
address & timestamp fields in the future (see ADR-25).

Special thanks to external contributors on this release:
@erikgrinaker, @PSalant726, @gchaincl, @gregzaitsev

Friendly reminder, we have a [bug bounty
program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

- CLI/RPC/Config

  - [rpc] \#3471 Paginate `/validators` response (default: 30 vals per page)
  - [rpc] \#3188 Remove `BlockMeta` in `ResultBlock` in favor of `BlockId` for `/block`
  - [rpc] `/block_results` response format updated (see RPC docs for details)
    ```
    {
      "jsonrpc": "2.0",
      "id": "",
      "result": {
        "height": "2109",
        "txs_results": null,
        "begin_block_events": null,
        "end_block_events": null,
        "validator_updates": null,
        "consensus_param_updates": null
      }
    }
    ```
  - [rpc] [\#4141](https://github.com/tendermint/tendermint/pull/4141) Remove `#event` suffix from the ID in event responses.
    `{"jsonrpc": "2.0", "id": 0, "result": ...}`
  - [rpc] [\#4141](https://github.com/tendermint/tendermint/pull/4141) Switch to integer IDs instead of `json-client-XYZ`
    ```
    id=0 method=/subscribe
    id=0 result=...
    id=1 method=/abci_query
    id=1 result=...
    ```
    - ID is unique for each request;
    - Request.ID is now optional. Notification is a Request without an ID. Previously ID="" or ID=0 were considered as notifications.

  - [config] \#4046 Rename tag(s) to CompositeKey & places where tag is still present it was renamed to event or events. Find how a compositeKey is constructed [here](https://github.com/tendermint/tendermint/blob/6d05c531f7efef6f0619155cf10ae8557dd7832f/docs/app-dev/indexing-transactions.md)
    - You will have to generate a new config for your Tendermint node(s)

- Apps

  - [tm-bench] Removed tm-bench in favor of [tm-load-test](https://github.com/interchainio/tm-load-test)

- Go API

  - [rpc/client] \#3471 `Validators` now requires two more args: `page` and `perPage`
  - [libs/common] \#3262 Make error the last parameter of `Task` (@PSalant726)
  - [cs/types] \#3262 Rename `GotVoteFromUnwantedRoundError` to `ErrGotVoteFromUnwantedRound` (@PSalant726)
  - [libs/common] \#3862 Remove `errors.go` from `libs/common`
  - [libs/common] \#4230 Move `KV` out of common to its own pkg
  - [libs/common] \#4230 Rename `cmn.KVPair(s)` to `kv.Pair(s)`s
  - [libs/common] \#4231 Move random functions from `libs/common` into pkg `rand`


- Blockchain Protocol

  - [abci] \#2521 Remove `TotalTxs` and `NumTxs` from `Header`
  - [types] [\#4151](https://github.com/tendermint/tendermint/pull/4151) Enforce ordering of votes in DuplicateVoteEvidence to be lexicographically sorted on BlockID
  - [types] \#1648 Change `Commit` to consist of just signatures

- P2P Protocol

  - [p2p] [\#3668](https://github.com/tendermint/tendermint/pull/3668) Make `SecretConnection` non-malleable

- [proto] [\#3986](https://github.com/tendermint/tendermint/pull/3986) Prefix protobuf types to avoid name conflicts.
  - ABCI becomes `tendermint.abci.types` with the new API endpoint `/tendermint.abci.types.ABCIApplication/`
  - core_grpc becomes `tendermint.rpc.grpc` with the new API endpoint `/tendermint.rpc.grpc.BroadcastAPI/`
  - merkle becomes `tendermint.crypto.merkle`
  - libs.common becomes `tendermint.libs.common`
  - proto3 becomes `tendermint.types.proto3`

### FEATURES:

- [p2p] \#4053 Add `unconditional_peer_ids` and `persistent_peers_max_dial_period` config variables (see ADR-050) (@dongsam)

### IMPROVEMENTS:

- [rpc] \#3188 Added `block_size` to `BlockMeta` this is reflected in `/blockchain`
- [types] \#2521 Add `NumTxs` to `BlockMeta` and `EventDataNewBlockHeader`
- [docs] [\#4111](https://github.com/tendermint/tendermint/issues/4111) Replaced dead whitepaper link in README.md
- [p2p] [\#4185](https://github.com/tendermint/tendermint/pull/4185) Simplify `SecretConnection` handshake with merlin
- [cli] [\#4065](https://github.com/tendermint/tendermint/issues/4065) Add `--consensus.create_empty_blocks_interval` flag (@jgimeno)
- [docs] [\#4065](https://github.com/tendermint/tendermint/issues/4065) Document `--consensus.create_empty_blocks_interval` flag (@jgimeno)
- [crypto] [\#4190](https://github.com/tendermint/tendermint/pull/4190) Added SR25519 signature scheme
- [abci] \#4177 kvstore: Return `LastBlockHeight` and `LastBlockAppHash` in `Info` (@princesinha19)

### BUG FIXES:

- [rpc/lib][\#4051](https://github.com/tendermint/tendermint/pull/4131) Fix RPC client, which was previously resolving https protocol to http (@yenkhoon)
- [rpc] [\#4141](https://github.com/tendermint/tendermint/pull/4141) JSONRPCClient: validate that Response.ID matches Request.ID
- [rpc] [\#4141](https://github.com/tendermint/tendermint/pull/4141) WSClient: check for unsolicited responses
- [types] [\4164](https://github.com/tendermint/tendermint/pull/4164) Prevent temporary power overflows on validator updates
- [cs] \#4069 Don't panic when block meta is not found in store (@gregzaitsev)
- [types] \#4164 Prevent temporary power overflows on validator updates (joint
  efforts of @gchaincl and @ancazamfir)
- [p2p] \#4140 `SecretConnection`: use the transcript solely for authentication (i.e. MAC)
