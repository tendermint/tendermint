Validators
==========

Validators are responsible for committing new blocks in the blockchain.
These validators participate in the consensus protocol by broadcasting
*votes* which contain cryptographic signatures signed by each
validator's public key.

Some Proof-of-Stake consensus algorithms aim to create a "completely"
decentralized system where all stakeholders (even those who are not
always available online) participate in the committing of blocks.
Tendermint has a different approach to block creation. Validators are
expected to be online, and the set of validators is permissioned/curated
by some external process. Proof-of-stake is not required, but can be
implemented on top of Tendermint consensus. That is, validators may be
required to post collateral on-chain, off-chain, or may not be required
to post any collateral at all.

Validators have a cryptographic key-pair and an associated amount of
"voting power". Voting power need not be the same.

Becoming a Validator
--------------------

There are two ways to become validator.

1. They can be pre-established in the `genesis
   state <./genesis.html>`__
2. The `ABCI app responds to the EndBlock
   message <https://github.com/tendermint/abci>`__ with changes to the
   existing validator set.

Committing a Block
------------------

*+2/3 is short for "more than 2/3"*

A block is committed when +2/3 of the validator set sign `precommit
votes <./block-structure.html#vote>`__ for that block at the same
``round``. The +2/3 set of precommit votes is
called a `*commit* <./block-structure.html#commit>`__. While any
+2/3 set of precommits for the same block at the same height&round can
serve as validation, the canonical commit is included in the next block
(see `LastCommit <./block-structure.html>`__).
