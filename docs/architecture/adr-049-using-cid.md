# ADR 049: Using IPFS CIDs instead of Hash

## Changelog

* 11-09-2019: Initial draft

## Context

Currently Tendermint is using hash bytes array for referring a block. However IPFS introduced CID for pointing materials.
Using CIDs will help to integrate blocks and transaction easier with IPFS network. At the same time we can use IPFS to query transactions or blocks with their CIDs.


## Decision

## Status

## Consequences


### Copyright

Copyright and related rights waived via [CC0](https://creativecommons.org/publicdomain/zero/1.0/).


## References
* [Content Identifiers (CIDs)](https://docs.ipfs.io/guides/concepts/cid/)
