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
[merkle proofs](https://github.com/tendermint/tendermint/blob/953523c3cb99fdb8c8f7a2d21e3a99094279e9de/spec/blockchain/encoding.md#iavl-tree).

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

Information on how to run a light client is located in the [nodes section](../nodes/light-client.md).
