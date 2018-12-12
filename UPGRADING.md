# Upgrading Tendermint Core

This guide provides steps to be followed when you upgrade your applications to
a newer version of Tendermint Core.

## v0.27.0

This release contains some breaking changes to the block and p2p protocols,
but does not change any core data structures, so it should be compatible with
existing blockchains from the v0.26 series that only used Ed25519 validator keys.
Blockchains using Secp256k1 for validators will not be compatible. This is due
to the fact that we now enforce which key types validators can use as a
consensus param. The default is Ed25519, and Secp256k1 must be activated
explicitly.

It is recommended to upgrade all nodes at once to avoid incompatibilities at the
peer layer - namely, the heartbeat consensus message has been removed (only
relevant if `create_empty_blocks=false` or `create_empty_blocks_interval > 0`),
and the proposer selection algorithm has changed. Since proposer information is
never included in the blockchain, this change only affects the peer layer.

### Go API Changes

#### libs/db

The ReverseIterator API has changed the meaning of `start` and `end`.
Before, iteration was from `start` to `end`, where
`start > end`. Now, iteration is from `end` to `start`, where `start < end`.
The iterator also excludes `end`. This change allows a simplified and more
intuitive logic, aligning the semantic meaning of `start` and `end` in the
`Iterator` and `ReverseIterator`.

### Applications

This release enforces a new consensus parameter, the
ValidatorParams.PubKeyTypes. Applications must ensure that they only return
validator updates with the allowed PubKeyTypes. If a validator update includes a
pubkey type that is not included in the ConsensusParams.Validator.PubKeyTypes,
block execution will fail and the consensus will halt.

By default, only Ed25519 pubkeys may be used for validators. Enabling
Secp256k1 requires explicit modification of the ConsensusParams.
Please update your application accordingly (ie. restrict validators to only be
able to use Ed25519 keys, or explicitly add additional key types to the genesis
file).

## v0.26.0

This release contains a lot of changes to core data types and protocols. It is not
compatible to the old versions and there is no straight forward way to update
old data to be compatible with the new version.

To reset the state do:

```
$ tendermint unsafe_reset_all
```

Here we summarize some other notable changes to be mindful of.

### Config Changes

All timeouts must be changed from integers to strings with their duration, for
instance `flush_throttle_timeout = 100` would be changed to
`flush_throttle_timeout = "100ms"` and `timeout_propose = 3000` would be changed
to `timeout_propose = "3s"`.

### RPC Changes

The default behaviour of `/abci_query` has been changed to not return a proof,
and the name of the parameter that controls this has been changed from `trusted`
to `prove`. To get proofs with your queries, ensure you set `prove=true`.

Various version fields like `amino_version`, `p2p_version`, `consensus_version`,
and `rpc_version` have been removed from the `node_info.other` and are
consolidated under the tendermint semantic version (ie. `node_info.version`) and
the new `block` and `p2p` protocol versions under `node_info.protocol_version`.

### ABCI Changes

Field numbers were bumped in the `Header` and `ResponseInfo` messages to make
room for new `version` fields. It should be straight forward to recompile the
protobuf file for these changes.

#### Proofs

The `ResponseQuery.Proof` field is now structured as a `[]ProofOp` to support
generalized Merkle tree constructions where the leaves of one Merkle tree are
the root of another. If you don't need this functionality, and you used to
return `<proof bytes>` here, you should instead return a single `ProofOp` with
just the `Data` field set:

```
[]ProofOp{
    ProofOp{
        Data: <proof bytes>,
    }
}
```

For more information, see:

- [ADR-026](https://github.com/tendermint/tendermint/blob/30519e8361c19f4bf320ef4d26288ebc621ad725/docs/architecture/adr-026-general-merkle-proof.md)
- [Relevant ABCI
  documentation](https://github.com/tendermint/tendermint/blob/30519e8361c19f4bf320ef4d26288ebc621ad725/docs/spec/abci/apps.md#query-proofs)
- [Description of
  keys](https://github.com/tendermint/tendermint/blob/30519e8361c19f4bf320ef4d26288ebc621ad725/crypto/merkle/proof_key_path.go#L14)

### Go API Changes

#### crypto/merkle

The `merkle.Hasher` interface was removed. Functions which used to take `Hasher`
now simply take `[]byte`. This means that any objects being Merklized should be
serialized before they are passed in.

#### node

The `node.RunForever` function was removed. Signal handling and running forever
should instead be explicitly configured by the caller. See how we do it
[here](https://github.com/tendermint/tendermint/blob/30519e8361c19f4bf320ef4d26288ebc621ad725/cmd/tendermint/commands/run_node.go#L60).

### Other

All hashes, except for public key addresses, are now 32-bytes.

## v0.25.0

This release has minimal impact.

If you use GasWanted in ABCI and want to enforce it, set the MaxGas in the genesis file (default is no max).

## v0.24.0

New 0.24.0 release contains a lot of changes to the state and types. It's not
compatible to the old versions and there is no straight forward way to update
old data to be compatible with the new version.

To reset the state do:

```
$ tendermint unsafe_reset_all
```

Here we summarize some other notable changes to be mindful of.

### Config changes

`p2p.max_num_peers` was removed in favor of `p2p.max_num_inbound_peers` and
`p2p.max_num_outbound_peers`.

```
# Maximum number of inbound peers
max_num_inbound_peers = 40

# Maximum number of outbound peers to connect to, excluding persistent peers
max_num_outbound_peers = 10
```

As you can see, the default ratio of inbound/outbound peers is 4/1. The reason
is we want it to be easier for new nodes to connect to the network. You can
tweak these parameters to alter the network topology.

### RPC Changes

The result of `/commit` used to contain `header` and `commit` fields at the top level. These are now contained under the `signed_header` field.

### ABCI Changes

The header has been upgraded and contains new fields, but none of the existing
fields were changed, except their order.

The `Validator` type was split into two, one containing an `Address` and one
containing a `PubKey`. When processing `RequestBeginBlock`, use the `Validator`
type, which contains just the `Address`. When returning `ResponseEndBlock`, use
the `ValidatorUpdate` type, which contains just the `PubKey`.

### Validator Set Updates

Validator set updates returned in ResponseEndBlock for height `H` used to take
effect immediately at height `H+1`. Now they will be delayed one block, to take
effect at height `H+2`. Note this means that the change will be seen by the ABCI
app in the `RequestBeginBlock.LastCommitInfo` at block `H+3`. Apps were already
required to maintain a map from validator addresses to pubkeys since v0.23 (when
pubkeys were removed from RequestBeginBlock), but now they may need to track
multiple validator sets at once to accomodate this delay.


### Block Size

The `ConsensusParams.BlockSize.MaxTxs` was removed in favour of
`ConsensusParams.BlockSize.MaxBytes`, which is now enforced. This means blocks
are limitted only by byte-size, not by number of transactions.
