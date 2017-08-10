# Light Client Protocol

Light clients are an important part of the complete blockchain system for most applications.  Tendermint provides unique speed and security properties for light client applications.

See our developing [light-client repository](https://github.com/tendermint/light-client).

## Overview

The objective of the light client protocol is to get a [commit](/docs/specs/validators#committing-a-block) for a recent [block hash](/docs/specs/block-structure#block-hash) where the commit includes a majority of signatures from the last known validator set.  From there, all the application state is verifiable with [merkle proofs](/docs/specs/merkle-trees#iavl-tree).

## Properties

- You get the full collateralized security benefits of Tendermint;  No need to wait for confirmations.
- You get the full speed benefits of Tendermint;  Transactions commit instantly.
- You can get the most recent version of the application state non-interactively (without committing anything to the blockchain).  For example, this means that you can get the most recent value of a name from the name-registry without worrying about fork censorship attacks, without posting a commit and waiting for confirmations.  It's fast, secure, and free!
