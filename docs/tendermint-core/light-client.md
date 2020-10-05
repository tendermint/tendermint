---
order: 13
---

# Light Client

Light clients are an important part of the complete blockchain system for most
applications. Tendermint provides unique speed and security properties for
light client applications.

See our [light
package](https://pkg.go.dev/github.com/tendermint/tendermint/light?tab=doc).

## Overview

The light client protocol verifies headers by retrieving a chain of headers,
commits and validator sets from a trusted height to the target height, verifying
the signatures of each of these intermediary signed headers till it reaches the
target height. From there, all the application state is verifiable with
[merkle proofs](https://github.com/tendermint/spec/blob/953523c3cb99fdb8c8f7a2d21e3a99094279e9de/spec/blockchain/encoding.md#iavl-tree).

## Properties

- You get the full collateralized security benefits of Tendermint; no
  need to wait for confirmations.
- You get the full speed benefits of Tendermint; transactions
  commit instantly.
- You can get the most recent version of the application state
  non-interactively (without committing anything to the blockchain). For
  example, this means that you can get the most recent value of a name from the
  name-registry without worrying about fork censorship attacks, without posting
  a commit and waiting for confirmations. It's fast, secure, and free!

## Security

A light client is initialized from a point of trust using [Trust Options](https://pkg.go.dev/github.com/tendermint/tendermint/light?tab=doc#TrustOptions),
a provider and a set of witnesses. This sets the trust period: the period that
full nodes should be accountable for faulty behavior and a trust level: the
fraction of validators in a validator set with which we trust that at least one
is correct. As Tendermint consensus can withstand 1/3 byzantine faults, this is
the default trust level, however, for greater security you can increase it (max:  
1).

Similar to a full node, light clients can also be subject to byzantine attacks.
A light client also runs a detector process which cross verifies headers from a
primary with witnesses. Therefore light clients should be set with enough witnesses.

If the light client observes a faulty provider it will report it to another provider
and return an error.

In summary, the light client is not safe when a) more than the trust level of
validators are malicious and b) all witnesses are malicious.

## Where to obtain trusted height & hash

One way to obtain a semi-trusted hash & height is to query multiple full nodes
and compare their hashes:

```bash
$ curl -s https://233.123.0.140:26657:26657/commit | jq "{height: .result.signed_header.header.height, hash: .result.signed_header.commit.block_id.hash}"
{
  "height": "273",
  "hash": "188F4F36CBCD2C91B57509BBF231C777E79B52EE3E0D90D06B1A25EB16E6E23D"
}
```

## Running a light client as an HTTP proxy server

Tendermint comes with a built-in `tendermint light` command, which can be used
to run a light client proxy server, verifying Tendermint RPC. All calls that
can be tracked back to a block header by a proof will be verified before
passing them back to the caller. Other than that, it will present the same
interface as a full Tendermint node.

You can start the light client proxy server by running `tendermint light <chainID>`,
with a variety of flags to specify the primary node,  the witness nodes (which cross-check
the information provided by the primary), the hash and height of the trusted header,
and more.

For example:

```bash
$ tendermint light supernova -p tcp://233.123.0.140:26657 \
  -w tcp://179.63.29.15:26657,tcp://144.165.223.135:26657 \
  --height=10 --hash=37E9A6DD3FA25E83B22C18835401E8E56088D0D7ABC6FD99FCDC920DD76C1C57
```

For additional options, run `tendermint light --help`.
