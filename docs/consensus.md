Tendermint Consensus version 0.7

Consensus in tendermint happens one block at a time: once a block is committed, it is never reverted.

Attempts to commit blocks happen in rounds. At a given height, if validators fail to commit a block, they move to the next round.

Rounds happen in steps. The steps allow validators to vote on blocks.

There are two kinds of votes: prevotes and precommits.

A block is committed when +2/3 of validators precommit for that block.

We want to ensure two main properties:

- Safety: the network should never have +2/3 validators precommit more than one block at the same height
- Liveness: the network should eventually get +2/3 precommits for one block at a given height 

These conditions may be violated if +1/3 of validators are byzantine (safety), or if +1/3 of validators are offline (liveness).

To achieve these properties, we introduce a consensus strategy which allows validators to discover what other validators think, and to make justifiably revokable commitments to one another, and hence to reach consensus. 

Finally, we would like our protocol to have some accountability gaurantees, such that if malicious behaviour is detected, the responsible validators can be identified.

Note that actually punishing malicious behaviour may require enforcement by clients (ie. clients need be willing to switch blockchains if malicious behaviour is observed on a previous chain).

--------------------

Tendermint satisfies these properties with the following protocol.

## Proposals

* Validators take turns proposing blocks at each round according to their voting power (more details ...)

## Prevotes

* Validators publish Prevotes for the proposals they've seen. Each validator may only prevote for one block at a given height/round.

* If a validator prevotes for more than one block at a given height/round, they are slashed

* If +2/3 of validators prevote for a particular block in the same round, it is evidence that the network is ready to commit a block.

* We call +2/3 prevotes for a single block in the same round a Polka. Indeed, the validators are doing the polka.

## Precommits

* We call +2/3 precommits for a single block in the same round a Commit. 

* If a validator precommits for more than one block at a given height/round, they are slashed

* A validator should only submit a precommit for a given block at some height/round if they have seen a Polka for that block at the same height. Otherwise, the validator must precommit nil after getting +2/3 prevotes (but not for a particular block, so not a Polka).

## Locks

* When a validator precommits a block, we say they are "locked" on that block.

* Once locked, a validator must prevote for the locked block in future rounds. 

* We cannot enforce slashing of a validator who prevotes differently from their lock without using an additional, probably overcomplicated, challenge response protocol to determine that the validator didn't unlock before prevoting. In the event this does happen, it will be resolved in the recovery protocol.

* Once locked, a validator must precommit nil for each subsequent round, unless they see another polka. Then they should precommit for the polka block.

* We cannot enforce slashing of a validator who precommits without evidence because precommits are not required to include evidence of the polka they are based on (this would be too much overhead). This means anyone can precommit without seeing a polka, but they cannot precommit twice (double sign). Precomitting without a polka will be resolved in the recovery protocol.

## Network Locks

* If +1/3 validators precommits for a given block at a height/round, the network is then locked on the block with a polka from that round.

Proof: Assume +2/3 are non byzantine.
Since validators must prevote what they are locked on, if +1/3 validators are locked and prevoting a given block, there will never be a polka for another block.
Since there can never be a polka for another block, the network is forced to commit the block in the aformentioned polka.

## Accountability

* If the blockchain forked, then either +1/3 of the validators double signed or +1/3 of the validators violated the rules of locking (either by precomitting without a valid polka, or prevoting differently from the block they are locked on).

Proof: Consider two blocks B1 and B2 to have been committed at rounds R1 and R2. We divide the proofs into two cases: R1 = R2 and R1 != R2

If R1 = R2 = R, then +2/3 of validators voted for each block in R. But (+2/3) + (+2/3) = (+4/3), which is greater than one. The difference (ie. (+4/3) - 1 = +1/3) must have double signed in R.

Suppose R1 != R2.  Without loss of generality, suppose R1 &lt; R2.

We will construct a proof by contradiction.  Suppose that no +1/3 of the validators had double signed _and_ no +1/3 of the validators had violated the rules of locking, and arrive at a contradiction.

Since no +1/3 of the validators had double signed, no round can have a polka for two different blocks.

Furthermore, no +1/3 of the validators had violated the rules of locking, so there can be no polka other than for B1, since at least +1/3 of validators are locked on B1 from R1 onwards.

Yet we have a fork, which means that either we had a polka for B2, or +2/3 validators had precommitted B2 without a polka for B2.  Either way we have a contradiction.

QED.

## Recovery

In the event of a fork (a violation of safety), we necessarily have double signing or violation of locking rules (ie. see above proof). 
The double signers should be slashed on both chains. However, since there are +1/3 byzantine, they may censor the evidence, and so an external recovery protocol must be invoked in which everyone publishes justification of their prevotes (previous precomits) and their precommits (polkas). 
Anyone without a valid polka for their precommit, or with a prevote that conflicts with their last precommit, may be considered byzantine, and a new blockchain should be started without them.

In a future version of Tendermint, we may modify the definition of a commit to be the full justification set of the +2/3 precommits for a single block at the height/round.  A justification set is a set of votes where all votes can be justified by the votes within the set.  This may simplify the recovery procedure by removing the phase needed to share justifications; rather, the justification sharing would be interleaved with the rounds, and each node would not consider a block to be committed until fully justified.
