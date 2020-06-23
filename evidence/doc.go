/*
Package evidence handles all evidence storage and gossiping from detection to block proposal.
For the different types of evidence refer to the `evidence.go` file in the types package
or https://github.com/tendermint/spec/blob/master/spec/consensus/light-client/accountability.md.

Gossiping

The core functionality begins with the evidence reactor (see reactor.
go) which operates both the sending and receiving of evidence.

The `Receive` function takes a list of evidence and does the following:

1. Breaks it down into individual evidence if it is `Composite Evidence`
(see types/evidence.go#ConflictingHeadersEvidence)

2. Checks that it does not already have the evidence stored

3. Verifies the evidence against the node's state (see state/validation.go#VerifyEvidence)

4. Stores the evidence to a db and a concurrent list

The gossiping of evidence is initiated when a peer is added which starts a go routine to broadcast currently
uncommitted evidence at intervals of 60 seconds (set by the by broadcastEvidenceIntervalS).
It uses a concurrent list to store the evidence and before sending verifies that each evidence is still valid in the
sense that it has not exceeded the max evidence age and height (see types/params.go#EvidenceParams).

Three are four buckets that evidence can be stored in: Pending, Committed, Awaiting and POLC's.

1. Pending is awaiting to be committed (evidence is usually broadcasted then)

2. Committed is for those already on the block and is to ensure that evidence isn't submitted twice

3. AwaitingTrial primarily refers to PotentialAmnesiaEvidence which must wait for a trial period before
being ready to be submitted (see docs/architecture/adr-056)

4. POLC's store all the ProofOfLockChanges that the node has done as part of consensus. To change lock is to vote
for a different block in a later round. The consensus module calls `AddPOLC()` to add to this bucket.

All evidence is proto encoded to disk.

Proposing

When a new block is being proposed (in state/execution.go#CreateProposalBlock),
`PendingEvidence(maxNum)` is called to send up to the maxNum number of uncommitted evidence, from the evidence store,
prioritized in order of age. All evidence is checked for expiration.

When a node receives evidence in a block it will use the evidence module as a cache first to see if it has
already verified the evidence before trying to verify it again.

Once the proposed evidence is submitted,
the evidence is marked as committed and is moved from the broadcasted set to the committed set.
As a result it is also removed from the concurrent list so that it is no longer gossiped.

Minor Functionality

As all evidence (including POLC's) are bounded by an expiration date, those that exceed this are no longer needed
and hence pruned. Currently, only committed evidence in which a marker to the height that the evidence was committed
and hence very small is saved. All updates are made from the `Update(block, state)` function which should be called
when a new block is committed.

*/
package evidence
