/*
Package evidence handles all evidence storage and gossiping from detection to block proposal.
For the different types of evidence refer to the `evidence.go` file in the types package
or https://github.com/tendermint/spec/blob/master/spec/consensus/light-client/accountability.md.

## Gossiping

The core functionality begins with the evidence reactor (see reactor.
go) which operates both the sending and receiving of evidence.

The `Receive` function takes a list of evidence and does the following:
 1. breaks it down into individual evidence if it is `Composite Evidence` (see types/evidence.
go#ConflictingHeadersEvidence)
 2. checks that it does not already have the evidence stored
 3. verifies the evidence against the nodes state (see state/validation.go#VerifyEvidence)
 4. stores the evidence to a db and a concurrent list

The gossiping of evidence is initiated when a peer is added which starts a go routine to broadcast currently
uncommitted evidence at intervals of 60 seconds (set by the by broadcastEvidenceIntervalS).
It uses a concurrent list to store the evidence and before sending verifies that each evidence is still valid in the
sense that it has not exceeded the max evidence age and height. This should be set to be equal to the "trusting
period" (see types/params.go#EvidenceParams).

## Proposing

When a new block is being proposed (in state/execution.go#CreateProposalBlock),
`PendingEvidence(maxNum)` is called to send up to the maxNum number of uncommitted evidence, from the evidence store,
based on a priority that is a product of the age of the evidence and the voting power of the malicious validator.

Once the proposed evidence is submitted,
the evidence is marked as committed and is moved from the broadcasted set to the committed set (
the committed set is used to verify whether new evidence has actually already been submitted).
As a result it is also removed from the concurrent list so that it is no longer gossiped.

Last Update: 14/04/20
*/
package evidence
