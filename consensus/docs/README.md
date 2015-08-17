Tendermint Consensus

Consensus in tendermint happens one block at a time: once a block is committed, it is never reverted.

A block is committed when +2/3 of validators publish a Precommit for that block.

Attempts to commit blocks happen in rounds. At a given height, if validators fail to get +2/3 precommits on a single block, they move to the next round.

We want to ensure two main properties:

- Safety: the network should never have +2/3 validators precommit more than one block at the same height
- Liveness: the network should eventually get +2/3 precommits for one block at a given height 

These conditions are violated if +1/3 of validators are byzantine (safety), or if +1/3 of validators are offline (liveness).

To achieve these properties, we introduce a consensus strategy which allows validators to discover what other validators think, and to make (justifiably revokable) commitments to one another, and hence to reach consensus. 

Finally, we would like our protocol to have some accountability gaurantees, such that if malicious behaviour is detected, the responsible validators can be identified.

Note that actually punishing malicious behaviour may require enforcements by clients (ie. clients need be willing to switch blockchains if malicious behaviour is observed on a previous chain).

--------------------

Tendermint satisfies these properties with the following protocol.

## Proposals

* Validators take turn proposing blocks at each round according to their voting power (more details ...)

## Prevotes

* Validators publish Prevotes for the proposals they've seen. Each validator may only prevote for one block at a given height/round.

* If a validator prevotes for more than one block at a given height/round, they are slashed (TODO)

* If +2/3 of validators prevote for a particular block, it is evidence that the network is ready to commit a block.

* We call +2/3 prevotes for a single block a Polka. Indeed, the validators are doing the polka.

## Precommits

* We call +2/3 precommits for a single block a Commit. 

* If a validator precommits for more than one block at a given height/round, they are slashed (TODO)

* A validator should only submit a precommit for a given block at some height/round if they have seen a Polka for that block at the same or an earlier round, but later than the last round in which they published a precommit. Ie., to change his precommit, a validator should have evidence that since he made the precommit, the network has become ready to commit a different block.

## Locks

* When a validator precommits a block, we say they are "locked" on that block.

* Once locked, a validator must prevote for the locked block in future rounds. 

* If a validator prevotes for a different block than the one they are locked on, they are slashed (TODO)

* Once locked, a validator must precommit nil for each subsequent round, unless they see another polka. Then they should precommit for the polka block.

* We cannot enforce slashing of precommits based on locked blocks because precommits are not required to include evidence of the polka they are based on (this would be too much overhead). This means anyone can precommit without seeing a polka, but they cannot precommit twice (double sign), and if the chain forks, an external protocol may be invoked which requires everyone to justify their precommits with valid polkas.

## Network Locks

* If +1/3 validators precommits for a given block at a height/round, the network is then locked on the block whose polka is closest to that round. 

Proof: Assume +2/3 are non byzantine. Since validators must prevote what they are locked on, if +1/3 validators are locked and prevoting a given block, there will never be a polka for another block. Since there can never be a polka for another block, the network is forced to commit a block for which there has already been a polka. Note, however, that more than one block may get +1/3 of precommits, since validators can make precommits in a round based on polkas from earlier rounds. Example: say there is a polka for B1 in R1, but no validators precommit for B1 in R1. Then in R2, suppose there is a polka for B2. Suppose +1/3 precommit on B2 based on that polka, and another +1/3 precommit on B1 based on the polka in R1. In a future round, those that are locked on B1 will see the polka for B2 and thus be able to switch to B2, since R2 > R1. Hence, once +1/3 are locked, the network is locked on the block with the most recent polka. QED.

## Accountability

* If the blockchain forked, then either +1/3 of the validators double signed or +1/3 of the validators violated the rules of locking (either by precomitting without a valid polka, or prevoting differently from the block they are locked on).

Proof: Consider two blocks B1 and B2 to have been comitted at rounds R1 and R2. We divide the proofs into two cases: R1 = R2 and R1 != R2

If R1 = R2 = R, then +2/3 of validators voted for each block in R. But (+2/3) + (+2/3) = (+4/3), which is greater than one. The difference (ie. (+4/3) - 1 = +1/3) must have double signed in R.

Suppose R1 != R2, and let the corresponding polkas for the blocks occur in rounds P1 and P2, where P1 <= R1 and P2 <= R2 (ie a precommit can only come after or in the same round as its polka). Without loss of generality, suppose P1 <= P2. Consider the two cases P1 = P2 and P1 < P2.

If P1 = P2, then similar to having two commits at the same round, we have two polkas at the same round, each requiring +2/3 prevotes. This again leaves us with an excess of +1/3, which means +1/3 prevoted for two blocks at the same round, ie. double signed.

Suppose P1 < P2. Since P1 <= R1 and P1 < P2, we have two cases: either R1 < P2 or R1 >= P2.

If R1 < P2, it means +2/3 prevoted for B2 after +2/3 precommited for B1. This means +1/3 of those who precommited for B1 must have prevoted for B2, hence violating their lock.

If R1 >= P2, it means +2/3 precommited B1 after +2/3 prevoted B2. This is perfectly acceptable, since we know P1 occured. We must therefore show that R2 required double signing. We already know R2 >= P2, so we must consider two final cases: R2 < R1, and R1 < R2 (again, we've already looked at R1=R2).

If R2 < R1, we have an overal sequence P1 < P2 <= R2 < R1. It means +2/3 precommited on B1 after +2/3 precommited on B2, which implies that +1/3 of those who precommitted on B2 also precommitted on B1 afterwards. This would be okay if R2 < P1 <= R1, however, since P1 < R2, this violates the rules of unlocking.

If R2 > R1, we have an overall sequence P1 < P2 <= R1 < R2. It means +2/3 precommitted on B2 after +2/3 precommitted on B1, which implies that +1/3 of those who precommitted on B1 also precommitted on B2 afterwards. This would be okay if R1 < P2 <= R2, however, since P2 < R1, this violated the rules of unlocking.

QED.


## Recovery

* In the event of a fork (a violation of safety), we necessarily have double signing or violation of locking rules. The double signers should be slashed on both chains. However, since there are +1/3 of them, they may censor the evidence, and so an external protocol must be invoked in which everyone publishes evidence of the Polka they based their precommits on. Anyone without a valid polka for their precommit may be considered byzantine, and a new blockchain should be started without them.

