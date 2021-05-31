---
order: 1
parent:
  title: Fast Sync
  order: 6
---

# Fast Sync

Fast Sync is a block syncing protocol used to catchup to the head of a chain from a height. In short the blockchain reactors continuously and in parallel downloads blocks from peers in order to provide blocks to the application. There are two implementations of the protocol. The first is `bcv0`, this is the original implementation and `bcv2` which is the most recent implementation. Both versions can be used in a production node.

- [bcv0](./bcv0/README.md)
- [bcv2](./bcv2/README.md)
