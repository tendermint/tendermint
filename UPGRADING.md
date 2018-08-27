# Upgrading Tendermint Core

This guide provides steps to be followed when you upgrade your applications to
a newer version of Tendermint Core.

## Upgrading from 0.23.0 to 0.24.0

New 0.24.0 release contains a lot of changes to the state and types. It's not
compatible to the old versions.

To reset the state do:

```
$ tendermint unsafe_reset_all
```

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
as we want it to be easier for new nodes to connect to the network. You can
tweak these parameters to alter the network topology.
