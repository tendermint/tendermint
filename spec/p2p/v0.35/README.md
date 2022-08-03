# Peer-to-peer communication substrate - WIP

This document details the operation of the [`p2p`][p2p-package] package of
Tendermint, refactored in the `v0.35` release.

**This is a work in progress** ([#8935][issue]). The following files represent the current (unfinished) state of this documentation. It has been decided not to finish the documents at this point in time, but to publish them here in the current form for future reference.

- [Peer manager](./peer_manager.md): determines when a node should connect to a
  new peer, and which peer is preferred for establishing connections.
- [Router](./router.md): implements the actions instructed by the peer manager,
  and route messages between the local reactors and the remote peers.

[issue]: https://github.com/tendermint/tendermint/issues/8935
[p2p-package]: https://github.com/tendermint/tendermint/tree/v0.35.x/internal/p2p
