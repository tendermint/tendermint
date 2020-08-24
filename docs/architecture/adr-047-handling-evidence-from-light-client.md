# ADR 047: Handling evidence from light client

## Changelog
* 18-02-2020: Initial draft
* 24-02-2020: Second version
* 13-04-2020: Add PotentialAmnesiaEvidence and a few remarks
* 31-07-2020: Remove PhantomValidatorEvidence
* 14-08-2020: Introduce light traces (listed now as an alternative approach)
* 20-08-2020: Light client produces evidence when detected instead of passing to full node

## Context

The bisection method of header verification used by the light client exposes
itself to a potential attack if any block within the light clients trusted period has
a malicious group of validators with power that exceeds the light clients trust level 
(default is 1/3). To improve light client (and overall network) security, the light
client has a detector component that compares the verified header provided by the
primary against witness headers. This ADR outlines the decision that ensues when
the light client detector receives two conflicting headers

## Alternative Approaches

A previously discussed approach to handling evidence was to pass all the data that the 
light client had witnessed when it had observed diverging headers for the full node to 
process.This was known as a light trace and had the following structure: 

```go
type ConflictingHeadersTrace struct {
  Headers []*types.SignedHeader
}
```

This approach has the advantage of not requiring as much processing on the light 
client side in the event that an attack happens. Although, this is not a significant 
difference as the light client would in any case have to validate all the headers 
from both witness and primary. Using traces would consume a large amount of bandwidth
and adds a ddos vector to the full node.


## Decision

The light client will be divided into two components: a `Verifier` (either sequential or 
bisection) and a `Detector` (see [Informal's Detector](https://github.com/informalsystems/tendermint-rs/blob/master/docs/spec/lightclient/detection/detection.md))
. The detector will take the trace of headers from the primary and check it against all 
witnesses. For a witness with a diverging header, the detector will first verify the header
by bisecting through all the heights defined by the trace that the primary provided. If valid,
the light client will trawl through both traces and find the point of bifurcation where it
can proceed to extract any evidence (as is discussed in detail later). 

Upon successfully detecting the evidence, the light client will send it to both primary and
witness before halting. It will not send evidence to other peers nor continue to verify the
primary's header against any other header.


## Detailed Design

The verification process of the light client will start from a trusted header and use a bisectional
algorithm to verify up to a header at a given height. This becomes the verified header (does not 
mean that it is trusted yet). All headers in between are cached and known as intermediary headers.

The light client's detector takes the verified header and concurrently queries all its witnesses for
the header of the same height in order to compare hashes. In the event that these two hashes are 
different the detector proceeds to do the following:

1. Check that the trusted header is the same. Currently, they should not theoretically be different
because witnesses cannot be added and removed after the client is initialized. But we do this any way
as a sanity check. If this fails we have to drop the witness.

2. We begin to query and verify the witness's headers using bisection at the same heights of all the
intermediary headers of the primary. If bisection fails or the witness stops responding then 
we can call the witness faulty and drop it. 

3. We eventually reach a verified header by the witness which is not the same as the intermediary header.
This is the point of bifurcation (This could also be the last header).

*NOTE: If this is not the last header, we could continue to verify up to the witness's final header,
but this is unnecessary. We have all the data we need to investigate if an attack has occurred.*

4. Finding the point of bifurcation could have come about in two ways:

	i. The light client could directly jump via bisection of the witness's headers from one intermediary 
	header height to another
	
	ii. The light client needs to verify further headers in between the two intermediary heights. 
	
	For the first case, this implies, if it is a lunatic attack, that some validators are still in the canonical 
	validator set. 
	
	For the second case this implies either that the validators that were part of the lunatic attack are no longer 
	in the canonical validator set or that the witness has performed a lunatic attack.
	
	In both cases, the light client checks for lunatic attack against the primary and also for equivocation. 
	For the second case, the light client runs the same detector but now against the witness using the trace it provided as the input and the primary as the cross-checking provider.
	
*NOTE: There is also a even slimmer chance that both providers are performing lunatic validator attacks from
different cabal groups. In this situation, the light client will halt without producing valid evidence.*

### Checking for a Lunatic Attack.

To quickly summarize a lunatic attack, the bisection verification works on the principle that if over 1/3
of the validator power in the height I trusted has also voted for a future header I don't trust yet, 
then from the Tendermint safety guarantee of 1/3, at least one validator in there must 
not be faulty. This principle breaks down of course if there is a cabal anywhere within the trusted
period that is greater than the light clients trust level (which is default at 1/3). This means that
such a cabal can fool the light client into verifying almost any arbitrary header. The detector
is predominantly designed as a second layer to detect such an attack.

Lunatic evidence has the following data structure:

```go
type LunaticAttackEvidence struct {
	SignedHeader          *SignedHeader
	ValidatorSourceHeight int64
	Timestamp             time.Time
}
```

Notice that unlike `DuplicateVoteEvidence` this evidence groups all malicious validators together. 
Another point of note is that the height and time, which is critical for calculating expiration is
the header that the light client trusted not the header where the attack actually happened. This is
done because after the header that the light client trusted we can't be sure that the validators
are still in the set and hence are still bonded. 

A lunatic attack is leveraged from the common trusted header. This is the intermediary header that 
came directly before the divergent headers and must contain a 1/3 overlap of validators.

The two divergent headers must have at least one of the deterministically derived header fields different
from one another. This could be one of `ValidatorsHash`, `NextValidatorsHash`, `ConsensusHash`,
`AppHash`, and `LastResultsHash`. If all these fields are the same then the divergence is not a product
of a lunatic attack. If so then we can generate two `LunaticAttackEvidence`:

```go
LunaticAttackEvidence { // To be delivered to the primary
	SignedHeader          from the divergent header of the witness
	ValidatorSourceHeight the height of the common header
	Timestamp             the time of the common header
}
```

```go
LunaticAttackEvidence { // To be delivered to the witness
	SignedHeader          from the divergent header of the primary
	ValidatorSourceHeight the height of the common header
	Timestamp             the time of the common header
}
```

**IMPORTANT:** The light client cannot verify which validator set is the correct one 
(it could if it used sequential verification) hence it generates two pieces of evidence 
and sends them to the opposite provider. One of which will be invalid against the 
full nodes state and the other that will be valid and can be committed onto the chain.

### Checking for Equivocation

The light client then checks if there has been a full fork that has caused the 
divergence.

It first differentiates between a duplicate vote attack and an amnesia attack by
checking the round that the commit of the divergent headers happened. If they
are the same then it checks the individual votes and produces `DuplicateVoteEvidence`.
If it is not the same then it bundles the signed header into amnesia evidence
(see [ADR 056](https://github.com/tendermint/tendermint/blob/master/docs/architecture/adr-056-proving-amnesia-attacks.md)).

For both of these evidence, the light client send the same evidence to both
witness and primary.

It is also important here to outline that neither evidence might be detected. 
This is because the attack that caused the full fork might have happened at a 
height that is between the common header and the divergent headers and therefore 
it is possible that both validator sets diverged to the point where there is 
no remnants with which to create evidence from. See future work for how this 
could be solved.

The light fork variant of an amnesia attack known as a back to the past attack 
where validators in the set forge extra votes and append them to an earlier
round will be detected by virtue of the fact that any light attack must happen
in the heights that the light client verifies and not in between.

## Full Node Verification

When a full node receives evidence from the light client it will need to verify
it for itself before trying to commit it on chain. This process is outlined
[here](https://github.com/tendermint/spec/blob/master/spec/core/data_structures.md#duplicatevoteevidence). 

## Possible Future Work

The algorithm outlined above runs on the assumption that after providing the 
initial bisection proof, that the malicious primary won't want to collaborate 
any further with the light client when it realizes it has been detected.
However, in the case of a full fork it is possible that both primary and 
witness are honest nodes and thus willing to collaborate further together 
to resolve the issue that the fork might have occurred at a height in between the
bisections. In this case, if the light client doesn't detect a lunatic attack 
and observes that the validator sets of the two heights are not the same, then 
the light client can iteratively work backwards from the diverged header,
querying and validating both providers until it reaches the point of bifurcation
on the full fork. This is the point that the headers have the same `LastBlockID` 
but different hashes. Now the validator sets will be the same and we
can extract out either the amnesia or duplicate votes using the same method. 

There has also been the notion of simplifying the evidence by not breaking it down 
into it's individual components and having a generalized evidence that is formed 
from the signed headers and is gossiped around. This will however involve
more verification as each full node will have to verify for all types of attacks 
and as this would involve quite a shake up to the current implementation it has 
been left as possible future work.

Another area of work that could be beneficial to look into would be in-consensus 
detection of amnesia attacks in a similar manner to the detection of duplicate votes.

## Status

Proposed.

## Consequences

### Positive

* Light client has increased security against Lunatic attacks.
* Tendermint will be able to detect & punish new types of misbehavior (Amnesia 
and Back to the past attacks)
* Light clients connected to multiple full nodes can help full nodes notice a
fork faster
* Do not need intermediate data structures to encapsulate the malicious behavior

### Negative

* Breaking change on the light client from versions 0.33.8 and below. Previous 
versions will still send `ConflictingHeadersEvidence` but it won't be recognized
by the full node. Light clients will however still refuse the header and shut down.

### Neutral

## References

* [Fork accountability spec](https://github.com/tendermint/spec/blob/master/spec/consensus/light-client/accountability.md)
* [ADR 056: Proving amnesia attacks](https://github.com/tendermint/tendermint/blob/master/docs/architecture/adr-056-proving-amnesia-attacks.md)
* [Informal's Light Client Detector](https://github.com/informalsystems/tendermint-rs/blob/master/docs/spec/lightclient/detection/detection.md)

## Appendix A

If there is an actual fork (full fork), a full node may follow either one or
another branch. So both H1 or H2 can be considered committed depending on which
branch the full node is following. It's supposed to halt if it notices an
actual fork, but there's a small chance it doesn't.

## Appendix B

PhantomValidatorEvidence was used to capture when a validator that was still staked
(i.e. within the bonded period) but was not in the current validator set had voted for a block.

In later discussions it was argued that although possible to keep phantom validator
evidence, any case a phantom validator that could have the capacity to be involved
in fooling a light client would have to be aided by 1/3+ lunatic validators.

It would also be very unlikely that the new validators injected by the lunatic attack
would be validators that currently still have something staked.

Not only this but there was a large degree of extra computation required in storing all
the currently staked validators that could possibly fall into the group of being
a phantom validator. Given this, it was removed.
