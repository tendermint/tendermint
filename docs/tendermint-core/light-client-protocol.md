---
order: 9
---

# Light Client Protocol

Light clients are an important part of the complete blockchain system for most
applications. Tendermint provides unique speed and security properties for
light client applications.

See our [lite
package](https://pkg.go.dev/github.com/tendermint/tendermint/lite2?tab=doc).

## Overview

The objective of the light client protocol is to get a commit for a recent
block hash where the commit includes a majority of signatures from the last
known validator set. From there, all the application state is verifiable with
[merkle
proofs](https://github.com/tendermint/spec/blob/953523c3cb99fdb8c8f7a2d21e3a99094279e9de/spec/blockchain/encoding.md#iavl-tree).

## Properties

- You get the full collateralized security benefits of Tendermint; No
  need to wait for confirmations.
- You get the full speed benefits of Tendermint; transactions
  commit instantly.
- You can get the most recent version of the application state
  non-interactively (without committing anything to the blockchain). For
  example, this means that you can get the most recent value of a name from the
  name-registry without worrying about fork censorship attacks, without posting
  a commit and waiting for confirmations. It's fast, secure, and free!

## Where to obtain trusted height & hash?

https://pkg.go.dev/github.com/tendermint/tendermint/lite2?tab=doc#TrustOptions

One way to obtain semi-trusted hash & height is to query multiple full nodes
and compare their hashes:

```sh
$ curl -s https://233.123.0.140:26657:26657/commit | jq "{height: .result.signed_header.header.height, hash: .result.signed_header.commit.block_id.hash}"
{
  "height": "273",
  "hash": "188F4F36CBCD2C91B57509BBF231C777E79B52EE3E0D90D06B1A25EB16E6E23D"
}
```

## HTTP proxy

Tendermint comes with a built-in `tendermint lite` command, which can be used
to run a light client proxy server, verifying Tendermint rpc. All calls that
can be tracked back to a block header by a proof will be verified before
passing them back to the caller. Other than that, it will present the same
interface as a full Tendermint node.

```sh
$ tendermint lite supernova -p tcp://233.123.0.140:26657 \
  -w tcp://179.63.29.15:26657,tcp://144.165.223.135:26657 \
  --height=10 --hash=37E9A6DD3FA25E83B22C18835401E8E56088D0D7ABC6FD99FCDC920DD76C1C57
```

For additional options, run `tendermint lite --help`.
