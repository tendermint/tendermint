NOTE: this wiki is mostly deprecated and left for archival purposes. Please see the [documentation website](http://tendermint.readthedocs.io/en/master/) which is built from the [docs directory](https://github.com/tendermint/tendermint/tree/master/docs). Additional information about the specification can also be found in that directory.

Light clients are an important part of the complete blockchain system for most applications. Tendermint provides unique speed and security properties for light client applications.

## Overview

The objective of the light client protocol is to get a [commit](Validators#committing-a-block) for a recent [block hash](Block-Structure#block-hash) where the commit includes a majority of signatures from the last known validator set. From there, all the application state is verifiable with [merkle proofs](Merkle-Trees#iavl-tree).

### Syncing the Validator Set

[[https://github.com/tendermint/tendermint/wiki/Light-client-syncing-of-validator-changes]]

## Properties

- You get the full collateralized security benefits of Tendermint;  No need to wait for confirmations.
- You get the full speed benefits of Tendermint;  Transactions commit instantly.
- You can get the most recent version of the application state non-interactively (without committing anything to the blockchain). For example, this means that you can get the most recent value of a name from the name-registry without worrying about fork censorship attacks, without posting a commit and waiting for confirmations. It's fast, secure, and free!