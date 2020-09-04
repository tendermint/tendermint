# ADR 059: Evidence Composition and Lifecycle

## Changelog

- 04/09/2020: Initial Draft (Unabridged)

## Scope

This document is a draft designed to collate together and surface some predicaments involving evidence in Tendermint with the aim of trying to form a more harmonious
composition which hopefully shall be later captured in one or two ADR's. The scope does not extend to the verification nor detection of certain types of evidence but concerns itself mainly with the general form of evidence and its lifecycle. Finally, this draft is an unabridged version and may contain a large degree of detail that may need to be consolidated in the future.

## Background

For a long time `DuplicateVoteEvidence`, formed in the consensus engine, was the only evidence Tendermint had. It was produced whenever two votes from the same validator in the same round
was observed and thus it was made in a per validator way. It was predicted that there may come more forms of evidence and thus `DuplicateVoteEvidence` was used as the model for the `Evidence` interface and also for the form of the evidence data sent to the application. It is important to note that Tendermint concerns itself just with the detection and reporting of evidence and it is the responsibility of the application to exercise punishment.

```go
type Evidence interface { //existing
  Height() int64                                     // height of the equivocation
  Time() time.Time                                   // time of the equivocation
  Address() []byte                                   // address of the equivocating validator
  Bytes() []byte                                     // bytes which comprise the evidence
  Hash() []byte                                      // hash of the evidence
  Verify(chainID string, pubKey crypto.PubKey) error // verify the evidence
  Equal(Evidence) bool                               // check equality of evidence

  ValidateBasic() error
  String() string
}
```

```go
type DuplicateVoteEvidence struct {
  VoteA *Vote
  VoteB *Vote

  timestamp time.Time // taken from the block time
}
```

Tendermint has now introduced a new type of evidence to protect light clients from being attacked. This `LightClientAttackEvidence` is vastly different to `DuplicateVoteEvidence` in that it is physically a much different size containing a complete signed header and validator set. It is formed within the light client, not the consensus engine and requires a lot more information from state to verify (`VerifyLightClientAttack(commonHeader, trustedHeader *SignedHeader, commonVals *ValidatorSet)`  vs `VerifyDuplicateVote(chainID string, pubKey PubKey)`). Finally it batches validators together as opposed to having individual evidence. This evidence stretches the existing mould that was used to accomodate new types of evidence and has thus caused us to reconsider how evidence should be formatted and processed.

```go
type LightClientAttackEvidence struct { // proposed struct in spec
  ConflictingBlock *LightBlock
  CommonHeight int64
  Type  AttackType     // enum: {Lunatic|Equivocation|Amnesia}

  timestamp time.Time // taken from the block time at the common height
}

```

## Possible Approaches for Evidence Composition

### Individual framework

Evidence remains on a per validator basis. This causes the least disruption to the current processes but requires that we break `LightClientAttackEvidence` into several pieces of evidence for each malicious validator. This not only has performance consequences in that there are n times as many database operations and that the gossiping of evidence will require more bandwidth then necessary (by requiring a header for each piece) but it potentially impacts our ability to validate it. In batch form, the full node can run the same process the light client did to see that 1/3 validating power was present in both the common block and the conflicting block whereas this becomes more difficult to verify individually without opening the possibility that malicious validators forge evidence against innocent . Not only that, but `LightClientAttackEvidence` also deals with amnesia attacks which unfortunately have the characteristic where we know the set of validators involved but not the subset that were actually malicious (more to be said about this later). And finally splitting the evidence into individual pieces makes it difficult

#### An example of a possible implementation path

We would ignore amnesia evidence (as individually it's hard to make) and revert to the initial split we had before where `DuplicateVoteEvidence` is also used for light client equivocation attacks and thus we only need. We would also most likely need to remove `Verify` from the interface as this isn't really something that is necessary.

``` go
type LunaticEvidence struct { // individual lunatic attack
  header *Header
  commonHeight int64
  vote *Vote

  timestamp time.Time // once again taken from the block time at the height of the common header
}
```

### Hybrid Framework

We allow individual and batch evidence to coexist together, meaning that verification, broadcasting and committing is done depending on the evidence type. A hybrid model effectively undermines any possibility of generalization and Tendermint's evidence module will use switch statements and concrete types instead of interfaces to correctly process the evidence. One could argue that although we could see more evidence on the horizon, this won't be to the extent of justifying generalizing evidence and that each evidence can make it's own unique way through the pipeline and on to being committed to the chain and broadcasted to the application.

A hybrid framework (as well as a batch framework) also surfaces the problem of the interface between Tendermint and the application. Currently, we have one concrete type that captures the relevant information needed to communicate to the application. This concrete type is formed per validator and therefore makes it more difficult to assess cartel (multi-validator) attacks. This begs the question of whether Tendermint should, at the application level, break down the evidence for each of the validators or alter the ABCI data structure to include multiple validators that were involved at that height and thus better capture the full extent of the misbehavior.

Last important note is that we are closest to this implementation than any of the other two.

#### An example of a possible implementation path

We would have to change the interface as it's not possible for `LightClientAttackEvidence` to return a single `Address()` so instead we would have `Addresses()`. In addition the actual infringing validators aren't known in `LightClientAttackEvidence` until verification is done as we are taking the union between the validator set at the common height and at the conflicting height.

```go
type Evidence interface { //proposed
  Height() int64                                     // height of the equivocation
  Time() time.Time                                   // time of the equivocation
  Addresses() []Address                              // addresses of the equivocating validators
  Bytes() []byte                                     // bytes which comprise the evidence
  Hash() []byte                                      // hash of the evidence
  Equal(Evidence) bool                               // check equality of evidence

  ValidateBasic() error
  String() string
}
```

Most likely we would therefore need to internally save the infringing validators which would make it easier to form abci evidence once the evidence has been committed on to chain. The `BlockExecutor` shouldn't have too much evidence specific logic in forming the abci evidence. Hence `LightClientAttackEvidence` should look like:

```go
type LightClientAttackEvidence struct {
  ConflictingBlock *LightBlock
  CommonHeight  int64
  Timestamp  time.Time
  Addresses []Address // we need this data in order to know which validators were at fault.
}
```


### Batch Framework

The last approach of this category would be to consider batch only evidence. This works fine with `LightClientAttackEvidence` but would require alterations to `DuplicateVoteEvidence` which would most likely mean that the consensus would send conflicting votes to a buffer in the evidence module which would then wrap all the votes together per height before gossiping them to other nodes and trying to commit it on chain. At a glance this may improve IO and verification speed and perhaps more importantly grouping validators gives the application and Tendermint a better overview of the severity of the attack. However individual evidence has the advantage that it is easy to check if a node already has that evidence meaning we just need to check hashes to know that we've already verified this evidence before. Batching evidence would imply that each node may have a different combination of duplicate votes which may complicate things.


#### An example of a possible implementation path

`LightClientAttackEvidence` won't change but the evidence interface will need to look like the proposed one above and `DuplicateVoteEvidence` will need to change to encompass multiple double votes. A problem with batch evidence is that it needs to be unique to avoid people from submitting different permutations.

| I most likely have not captured the full picture of all these approaches and therefore would encourage others to add their takes on each of them.


## Evidence Interface

*Note: any changes to the interface will be changing the block data structure and is hence breaking*

As already been alluded to it is most likely that we will need to remove `Verify()` as it's difficult to define a set of arguments that will satisfy the requirements for both duplicate vote verification and light client attack verification (not to mention future evidence and their requirements).

Depending on the composition we might need to change `Address()` to accomodate either individual and/or batch evidence. It may also be a good idea to have internal fields that the evidence pool populates such as time and validators so modules outside of the evidence pool (specifically the `BlockExecutor` don't need to query for more resources)

`Equal()` can also be seen as redundant if we set `Hash()` in the right way that we can just compare evidence hashes for uniqueness. We already use hashes for database lookups

`Type()` could be a new method which would be helpful in constructing abci evidence

`ToABCI()` is another alternative that was written as a TODO in the actual code base.


## The State of Amnesia

TBC

## Censorship

A second problem brought up in a prior [issue](https://github.com/tendermint/tendermint/issues/4593) is the notion of evidence censorship. Consider the following scenario: I have greater than 1/3 validator power. I perform an attack either on a light client of on the main network that was detected. Whenever someone proposes a block that contains evidence I abstain from voting meaning that it never is committed. When it is my turn to propose, I propose a block that does not include the evidence, everyone else votes and that block gets committed. Eventually the chain reaches a height and time where the evidence has expired and any attempt to punish my attack has been thwarted.

This would most likely need an RFC (in order to explore possible solutions). As far as I can see there is a 0% approach a 50% approach and 100% approach in terms of the level of intervention.

The 0% approach would just be simply to leave this as is.

The 50% approach would be that validators would consider a proposal invalid if it didn't contain at least one of the evidence that that validator had and thus wouldn't vote on it. This solution is far less intrusive to the current consensus algorithm but it also doesn't quite solve the problem of censorship, rather this protocol will keep the chain halted until off-chain governance can intervene.

The 100% approach is a lot more intricate and would be to find a protocol where upon receiving a valid proposal with evidence, current validators that had been shown to have misbehaved would be barred from voting such that the total voting power would be reduced by the voting power of the malicious validators and this would allow for 2/3 consensus to be reached regardless of what the malicious validators did.


## Decision

To be made

> This section records the decision that was made.
> It is best to record as much info as possible from the discussion that happened. This aids in not having to go back to the Pull Request to get the needed information.

> ## Detailed Design

> This section does not need to be filled in at the start of the ADR, but must be completed prior to the merging of the implementation.
>
> Here are some common questions that get answered as part of the detailed design:
>
> - What are the user requirements?
>
> - What systems will be affected?
>
> - What new data structures are needed, what data structures will be changed?
>
> - What new APIs will be needed, what APIs will be changed?
>
> - What are the efficiency considerations (time/space)?
>
> - What are the expected access patterns (load/throughput)?
>
> - Are there any logging, monitoring or observability needs?
>
> - Are there any security considerations?
>
> - Are there any privacy considerations?
>
> - How will the changes be tested?
>
> - If the change is large, how will the changes be broken up for ease of review?
>
> - Will these changes require a breaking (major) release?
>
> - Does this change require coordination with the SDK or other?

## Status

> A decision may be "proposed" if it hasn't been agreed upon yet, or "accepted" once it is agreed upon. Once the ADR has been implemented mark the ADR as "implemented". If a later ADR changes or reverses a decision, it may be marked as "deprecated" or "superseded" with a reference to its replacement.

{Proposed}

## Consequences

> This section describes the consequences, after applying the decision. All consequences should be summarized here, not just the "positive" ones.

### Positive

### Negative

### Neutral

## References

> Are there any relevant PR comments, issues that led up to this, or articles referenced for why we made the given design choice? If so link them here!

- [LightClientAttackEvidence](https://github.com/informalsystems/tendermint-rs/blob/31ca3e64ce90786c1734caf186e30595832297a4/docs/spec/lightclient/attacks/evidence-handling.md)
