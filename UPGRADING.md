# Upgrading Tendermint Core

This guide provides steps to be followed when you upgrade your applications to
a newer version of Tendermint Core.

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
