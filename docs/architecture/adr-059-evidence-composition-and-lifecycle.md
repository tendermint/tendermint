# ADR 059: Evidence Composition and Lifecycle

## Changelog

- 04/09/2020: Initial Draft

## Scope

This document is a draft designed to collate together and surface some predicaments involving evidence in Tendermint with the aim of trying to form a more harmonious
composition which hopefully shall be later captured in one or two ADR's. The scope does not extend to the verification nor detection of certain types of evidence but concerns itself mainly with the general form of evidence and its lifecycle.

## Background

For a long time `DuplicateVoteEvidence`, formed in the consensus engine, was the only evidence Tendermint had. It was produced whenever two votes from the same validator in the same round
was observed and thus it was made in a per validator way. It was predicted that there may come more forms of evidence and thus `DuplicateVoteEvidence` was used as the model for the `Evidence` interface and also for the form of the evidence data sent to the application. It is important to note that Tendermint concerns itself just with the detection and reporting of evidence and it is the responsibility of the application to exercise punishment.

Tendermint has now introduced a new type of evidence to protect light clients from being attacked. This `LightClientAttackEvidence` is vastly different to `DuplicateVoteEvidence` in that it is physically a much different size containing a complete signed header and validator set. It is formed within the light client, not the consensus engine and requires a lot more information from state to verify. Finally it batches validators together as opposed to having individual evidence. This evidence stretches the existing mould that was used to accomodate new types of evidence and has thus caused us to reconsider how evidence should be formatted and processed.

## Possible Approaches

### Individual framework

Evidence remains on a per validator basis. This causes the least disruption to the current processes but requires that we break `LightClientAttackEvidence` into several pieces of evidence for each malicious validator. This not only has performance consequences in that there are n times as many database operations and that the gossiping of evidence will require more bandwidth then necessary (by requiring a header for each piece) but it potentially impacts our ability to validate it. In batch form, the full node can run the same process the light client did to see that 1/3 validating power was present in both the common block and the conflicting block whereas this becomes more difficult to verify individually. Not only that, but `LightClientAttackEvidence` also deals with amnesia attacks which unfortunately have the characteristic where we know the set of validators involved but not the subset that were actually malicious (more to be said about this later).

### Hybrid Framework

We allow individual and batch evidence to coexist together, meaning that verification, broadcasting and committing is done depending on the evidence type. A hybrid model effectively undermines any possibility of generalization and Tendermint's evidence module will use switch statements and concrete types instead of interfaces to correctly process the evidence. One could argue that although we could see more evidence on the horizon, this won't be to the extent of justifying generalizing evidence and that each evidence can make it's own unique way through the pipeline and on to being committed to the chain and broadcasted to the application.

A hybrid framework (as well as a batch framework) also surfaces the problem of the interface between Tendermint and the application. Currently, we have one concrete type that captures the relevant information needed to communicate to the application. This concrete type is formed per validator and therefore makes it more difficult to assess cartel (multi-validator) attacks. This begs the question of whether Tendermint should, at the application level, break down the evidence for each of the validators or alter the ABCI data structure to include multiple validators that were involved at that height and thus better capture the full extent of the misbehavior.

Last important note is that we are closest to this implementation than any of the other two.

### Batch Framework

The last approach of this category would be to consider batch only evidence. This works fine with `LightClientAttackEvidence` but would require alterations to `DuplicateVoteEvidence` which would most likely mean that the consensus would send conflicting votes to a buffer in the evidence module which would then wrap all the votes together per height before gossiping them to other nodes and trying to commit it on chain. At a glance this may improve IO and verification speed and perhaps more importantly grouping validators gives the application and Tendermint a better overview of the severity of the attack. However individual evidence has the advantage that it is easy to check if a node already has that evidence meaning we just need to check hashes to know that we've already verified this evidence before. Batching evidence would imply that each node may have a different combination of duplicate votes which may complicate things.

| I most likely have not captured the full picture of all these approaches and therefore would encourage others to add their takes on each of them.


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

- {reference link}
