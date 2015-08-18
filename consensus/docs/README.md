Tendermint Consensus

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

* A validator should only submit a precommit for a given block at some height/round if they have seen a Polka for that block at the same or an earlier round, but later than the last round in which they published a precommit. Ie., to change his precommit, a validator should have evidence that since he made the precommit, the network has become ready to commit a different block.

## Locks

* When a validator precommits a block, we say they are "locked" on that block.

* Once locked, a validator must prevote for the locked block in future rounds. 

* We cannot enforce slashing of a validator who prevotes differently from their lock without using an additional, probably overcomplicated, challenge response protocol to determine that the validator didn't unlock before prevoting. In the event this does happen, it will be resolved in the recovery protocol.

* Once locked, a validator must precommit nil for each subsequent round, unless they see another polka. Then they should precommit for the polka block.

* We cannot enforce slashing of a validator who precommits without evidence because precommits are not required to include evidence of the polka they are based on (this would be too much overhead). This means anyone can precommit without seeing a polka, but they cannot precommit twice (double sign). Precomitting without a polka will be resolved in the recovery protocol.

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

If R1 >= P2, it means +2/3 precommited B1 after +2/3 prevoted B2. This is perfectly acceptable, since we know P1 occured. We must therefore show that R2 required double signing or violates the rules of locking. We already know R2 >= P2, so we must consider two final cases: R2 < R1, and R1 < R2 (again, we've already looked at R1=R2).

If R2 < R1, we have an overal sequence P1 < P2 <= R2 < R1. It means +2/3 precommited on B1 after +2/3 precommited on B2, which implies that +1/3 of those who precommitted on B2 also precommitted on B1 afterwards. This would be okay if R2 < P1 <= R1, however, since P1 < R2, this violates the rules of unlocking.

If R2 > R1, we have an overall sequence P1 < P2 <= R1 < R2. It means +2/3 precommitted on B2 after +2/3 precommitted on B1, which implies that +1/3 of those who precommitted on B1 also precommitted on B2 afterwards. This would be okay if R1 < P2 <= R2, however, since P2 < R1, this violated the rules of unlocking.

QED.


## Recovery

* In the event of a fork (a violation of safety), we necessarily have double signing or violation of locking rules (ie. see above proof). 
The double signers should be slashed on both chains. However, since there are +1/3 byzantine, they may censor the evidence, and so an external recovery protocol must be invoked in which everyone publishes justification of their prevotes (previous precomits) and their precommits (polkas). 
Anyone without a valid polka for their precommit, or with a prevote that conflicts with their last precommit, may be considered byzantine, and a new blockchain should be started without them.

** Recovery Spec

Assume the blockchain has forked at height H, with blocks B1 and B2 produced at the same round R. We know this means +1/3 validators precommitted two different blocks at the same height/round. Therefore our task is easy: gossip the duplicate signatures, and produce a special block at height H and round Rr > R. The block should be proposed by the appropriate next proposer after removing the offending validators, and it should pass through the normal consensus protocol for being committed, but with the reduced set of validators (to exclude double signers). The block should contain no transactions, but rather a merkle tree of evidence for those that double signed. Once block H is committed, consensus resumes as normal at block H+1 with the reduced validator set. Light clients will have to do the hard work of iterating through the whole tree (processing the whole block) to cofirm how the validator set should be updated. 

Alternatively, we may wish to commit one of B1 or B2. In this case, the special block should be at H+1, and its proposer should include in the proposal B1 or B2. The special block should include validation from the reduced set of validators for B1 or B2.

Assume instead the blockchain has forked at height H, with blocks B1 and B2 produced at rounds R1 and R2, with R1 < R2. We know this means +1/3 validators double signed or violated locking rules, and now we need to figure out who they are. This can only be done through a publishing protocol where each validator publishes the polkas it used as justification for each precommit and/or prevote (if applicable - ie. if prevoting differently from last lock)

Imagine the following example: a polka for B1 is produced at P1, and (2/3+e) precommit for it. 
Suppose a polka for B2 is then produced at P2 by (1/3-e) not locked and an additional (1/3+f) that are, with f > e.
Then suppose that motley crew of (2/3+f-e > 2/3) precommits on B2.
The blockchain has forked and no one has double signed, but the (1/3+f) unjustifiably prevoted B2 when they shouldn't have.

We must determine who the lock violaters are, and start a new chain with out them, again using a special block. 
Producing this block securely is more difficult, since we must first offer validators a chance to publish their justifications.
The special block should include a new validator set with +1/2 of the original set.
The special block should include the conflicting validations for B1 and B2.
The special block should include every precommit from that height - these may be submitted by any validator.
The special block should include justification for each validator which precomitted B1 or B2. 
Justification should include a list of all non nil prevotes, precommits, and polkas, and should saitsfy the following:
Consider the list of non-nil precommits made by validator V as Cv = {c1, c2, ..., cN}, or as a function Cv(i), with i in [1, N].
V need not have a precommit at each round, but we assume to know every round for which he did.
V must publish a corresponding list of polkas Pv = {p1, p2, ..., pN}, or Pv(i), 
such that for each Cv(i).round there exists a Pv(i) such that Cv(i-1).round < Pv(i).round <= Cv(i).round. 
In other words, for each non-nil precommit at a round R, there must be a valid polka that happened at or before R, but after the last precommit.
Consider also the list of prevotes made by validator V as Qv = {q1, q2, ..., qM}. 
Note the validator may have published a different number of precommits and prevotes (M != N), but we assume to know all of both.
Now join Qv and Cv and order them according to their rounds. 
Any continuos sequence of prevotes following a precommit must be for the same block as that in the precommit.
Validators who cannot satisfy either of these conditions are excluded from the new validator set.
