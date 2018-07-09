I guess this is its own section under Tendermint Core?

We should explain how `lite` is expected to be used and the distinction
between what Tendermint can verify and what needs be verified by
application-specific logic. Eg the LCD uses the lite pkg to verify tendermint
headers and commits but then uses SDK code to verify merkle proofs of the state
of accounts


Light Client Protocol
=====================

Light clients are an important part of the complete blockchain system
for most applications. Tendermint provides unique speed and security
properties for light client applications.

See our `lite package
<https://godoc.org/github.com/tendermint/tendermint/lite>`__.

Overview
--------

The objective of the light client protocol is to get a
`commit <./validators.html#committing-a-block>`__ for a recent
`block hash <./block-structure.html#block-hash>`__ where the commit
includes a majority of signatures from the last known validator set.
From there, all the application state is verifiable with `merkle
proofs <./merkle.html#iavl-tree>`__.

Properties
----------

-  You get the full collateralized security benefits of Tendermint; No
   need to wait for confirmations.
-  You get the full speed benefits of Tendermint; transactions commit
   instantly.
-  You can get the most recent version of the application state
   non-interactively (without committing anything to the blockchain).
   For example, this means that you can get the most recent value of a
   name from the name-registry without worrying about fork censorship
   attacks, without posting a commit and waiting for confirmations. It's
   fast, secure, and free!
