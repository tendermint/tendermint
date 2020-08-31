# ADR 047: Handling evidence from light client

## Changelog
* 18-02-2020: Initial draft
* 24-02-2020: Second version
* 13-04-2020: Add PotentialAmnesiaEvidence and a few remarks
* 31-07-2020: Remove PhantomValidatorEvidence
* 14-08-2020: Introduce light traces

## Context

The bisection method of header verification used by the light client exposes
itself to a potential attack if any block within the light clients trusted period has
a malicious group of validators with power that exceeds the light clients trust level 
(default is 1/3). To improve light client (and overall network) security, the light
client has a detector component that compares the verified header provided by the
primary against witness headers. This ADR outlines the decision that ensues when
the light client detector receives two conflicting headers

## Alternative Approaches

One of the key arguments surrounding the decision is whether the processing required
to extract verifiable evidence should be on the light client side or the full node side.
As light client, indicated in it's name, is supposed to stay light, the natural 
inclination is to avoid any load and pass it directly to the full node who also has
the advantage of having a full state. It remains possible in future discussions to have the
light client form the evidence types itself. The other minor downsides apart from the
load will be that around half the evidence produced by the light client will be invalid and
that, in the event of modified logic, it's easier and expected that the full node be up 
to date than the light clients.

## Decision

When two headers have different hashes, the light client must first verify via 
bisection that the signed header provided by the witness is also valid.

It then collects all the headers that were fetched as part of bisection into two traces. 
One containing the primary's headers and the other containing the witness's headers. 
The light client sends the trace to the opposite provider (i.e. the primary's trace
to the witness and the witness's trace to the primary) as the light client is 
incapable of deciding which one is malicious. 

Assuming one of the two providers is honest, that full node will then take the trace
and extract out the relevant evidence and propogate it until it ends up on chain.

*NOTE: We do not send evidence to other connected peers*

*NOTE: The light client halts then and does not verify with any other witness*

## Detailed Design

The traces will have the following data structure: 

```go
type ConflictingHeadersTrace struct {
  Headers []*types.SignedHeader
}
```

When a full node receives a `ConflictingHeadersTrace`, it should
a) validate it b) figure out if malicious behaviour is obvious (immediately
slashable) or run the amnesia protocol.

### Validating headers

Check headers are valid (`ValidateBasic`), are in order of ascending height, and do not exceed the `MaxTraceSize`.

### Finding Block Bifurcation

The node pulls the block ID's for the respective heights of the trace headers from its own block store.

First it checks to see that the first header hash matches its first `BlockID` else it can discard it. 

If the last header hash matches the nodes last `BlockID` then it can also discard it on the assumption that a fork can not remerge and hence this is just a trace of valid headers. 

The node then continues to loop in descending order checking that the headers hash doesn't match it's own blockID for that height. Once it reaches the height that the block ID matches the hash it then sends the common header, the trusted header and the diverged header (common header is needed for lunatic evidence) to determine if the divergence is a real offense to the tendermint protocol or if it is just fabricated.

### Figuring out if malicious behaviour

The node first examines the case of a lunatic attack:

* The validator set of the common header must have at least 1/3 validator power that signed in the divergedHeaders commit

* One of the deterministically derived hashes (`ValidatorsHash`, `NextValidatorsHash`, `ConsensusHash`,
`AppHash`, or `LastResultsHash`) of the header must not match:

* We then take every validator that voted for the invalid header and was a validator in the common headers validator set and create `LunaticValidatorEvidence`

If this fails then we examine the case of Equivocation (either duplicate vote or amnesia):

*This only requires the trustedHeader and the divergedHeader*

* if `trustedHeader.Round == divergedHeader.Round`, and a validator signed for the block in both headers then DuplicateVoteEvidence can be immediately formed

* if `trustedHeader.Round != divergedHeader.Round` then we form PotentialAmnesiaEvidence as some validators in this set have behaved maliciously and protocol in ADR 56 needs to be run. 

*The node does not check that there is a 1/3 overlap between headers as this may not be point of the fork and validator sets may have since changed*

If no evidence can be formed from a light trace, it is not a legitimate trace and thus the 
connection with the peer should be stopped

### F1. Equivocation

Existing `DuplicateVoteEvidence` needs to be created and gossiped.


### F5. Lunatic validator

```go
type LunaticValidatorEvidence struct {
	Header             types.Header
	Vote               types.Vote
	InvalidHeaderField string
}
```

To punish this attack, we need support for a new Evidence type -
`LunaticValidatorEvidence`. This type includes a vote and a header. The header
must contain fields that are invalid with respect to the previous block, and a
vote for that header by a validator that was in a validator set within the
unbonding period. While the attack is only possible if +1/3 of some validator
set colludes, the evidence should be verifiable independently for each
individual validator. This means the total evidence can be split into one piece
of evidence per attacking validator and gossipped to nodes to be verified one
piece at a time, reducing the DoS attack surface at the peer layer.

Note it is not sufficient to simply compare this header with that committed for
the corresponding height, as an honest node may vote for a header that is not
ultimately committed. Certain fields may also be variable, for instance the
`LastCommitHash` and the `Time` may depend on which votes the proposer includes.
Thus, the header must be explicitly checked for invalid data.

For the attack to succeed, VC must sign a header that changes the validator set
to consist of something they control. Without doing this, they can not
otherwise attack the light client, since the client verifies commits according
to validator sets. Thus, it should be sufficient to check only that
`ValidatorsHash` and `NextValidatorsHash` are correct with respect to the
header that was committed at the corresponding height.

That said, if the attack is conducted by +2/3 of the validator set, they don't
need to make an invalid change to the validator set, since they already control
it. Instead they would make invalid changes to the `AppHash`, or possibly other
fields. In order to punish them, then, we would have to check all header
fields.

Note some header fields require the block itself to verify, which the light
client, by definition, does not possess, so it may not be possible to check
these fields. For now, then, `LunaticValidatorEvidence` must be checked against
all header fields which are a function of the application at previous blocks.
This includes `ValidatorsHash`, `NextValidatorsHash`, `ConsensusHash`,
`AppHash`, and `LastResultsHash`. These should all match what's in the header
for the block that was actually committed at the corresponding height, and
should thus be easy to check.

`InvalidHeaderField` contains the invalid field name. Note it's very likely
that multiple fields diverge, but it's faster to check just one. This field
MUST NOT be used to determine equality of `LunaticValidatorEvidence`.

### F2. Amnesia

```go
type PotentialAmnesiaEvidence struct {
	VoteA types.Vote
	VoteB types.Vote
}
```

To punish this attack, votes under question needs to be sent. Fork
accountability process should then use this evidence to request additional
information from offended validators and construct a new type of evidence to
punish those who conducted an amnesia attack.

See ADR-056 for the architecture of the handling amnesia attacks.

NOTE: Conflicting headers trace used to also create PhantomValidatorEvidence
but this has since been removed. Refer to Appendix B.  

## Status

Proposed.

## Consequences

### Positive

* Light client has increased secuirty against Lunatic attacks.
* Tendermint will be able to detect & punish new types of misbehavior
* light clients connected to multiple full nodes can help full nodes notice a
  fork faster

### Negative

* Accepting `ConflictingHeadersEvidence` from light clients opens up a large DDOS
attack vector(same is fair for any RPC endpoint open to public; remember that
RPC is not open by default).

### Neutral

## References

* [Fork accountability spec](https://github.com/tendermint/spec/blob/master/spec/consensus/light-client/accountability.md)
* [ADR 056: Proving amnesia attakcs](https://github.com/tendermint/tendermint/blob/master/docs/architecture/adr-056-proving-amnesia-attacks.md)

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
