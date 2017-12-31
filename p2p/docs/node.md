# Tendermint Peer Discovery

A Tendermint P2P network has different kinds of nodes with different requirements for connectivity to others.
This document describes what kind of nodes Tendermint should enable and how they should work.

## Node startup options
--p2p.seed_mode // If present, this node operates in seed mode.  It will kick incoming peers after sharing some peers.
--p2p.seeds “1.2.3.4:466656,2.3.4.5:4444” // Dials these seeds to get peers and disconnects.
--p2p.persistent_peers “1.2.3.4:46656,2.3.4.5:466656” // These connections will be auto-redialed.  If dial_seeds and persistent intersect, the user will be WARNED that seeds may auto-close connections and the node may not be able to keep the connection persistent

## Seeds

Seeds are the first point of contact for a new node.
They return a list of known active peers and disconnect.

Seeds should operate full nodes, and with the PEX reactor in a "crawler" mode
that continuously explores to validate the availability of peers.

Seeds should only respond with some top percentile of the best peers it knows about.

## New Full Node

A new node has seeds hardcoded into the software, but they can also be set manually (config file or flags).
The new node must also have access to a recent block height, H, and hash, HASH.

The node then queries some seeds for peers for its chain,
dials those peers, and runs the Tendermint protocols with those it successfully connects to.

When the peer catches up to height H, it ensures the block hash matches HASH.

## Restarted Full Node

A node checks its address book on startup and attempts to connect to peers from there.
If it can't connect to any peers after some time, it falls back to the seeds to find more.

## Validator Node

A validator node is a node that interfaces with a validator signing key.
These nodes require the highest security, and should not accept incoming connections.
They should maintain outgoing connections to a controlled set of "Sentry Nodes" that serve
as their proxy shield to the rest of the network.

Validators that know and trust each other can accept incoming connections from one another and maintain direct private connectivity via VPN.

## Sentry Node

Sentry nodes are guardians of a validator node and provide it access to the rest of the network.
Sentry nodes may be dynamic, but should maintain persistent connections to some evolving random subset of each other.
They should always expect to have direct incoming connections from the validator node and its backup/s.
They do not report the validator node's address in the PEX.
They may be more strict about the quality of peers they keep.

Sentry nodes belonging to validators that trust each other may wish to maintain persistent connections via VPN with one another, but only report each other sparingly in the PEX.
