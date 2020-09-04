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

We would ignore amnesia evidence (as individually it's hard to make) and revert to the initial split we had before where `DuplicateVoteEvidence` is also used for light client equivocation attacks and thus we only need `LunaticEvidence`. We would also most likely need to remove `Verify` from the interface as this isn't really something that can be used.

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

The addresses would also be used as a look up table such that we can store evidence per address.


### Batch Framework

The last approach of this category would be to consider batch only evidence. This works fine with `LightClientAttackEvidence` but would require alterations to `DuplicateVoteEvidence` which would most likely mean that the consensus would send conflicting votes to a buffer in the evidence module which would then wrap all the votes together per height before gossiping them to other nodes and trying to commit it on chain. At a glance this may improve IO and verification speed and perhaps more importantly grouping validators gives the application and Tendermint a better overview of the severity of the attack. However individual evidence has the advantage that it is easy to check if a node already has that evidence meaning we just need to check hashes to know that we've already verified this evidence before. Batching evidence would imply that each node may have a different combination of duplicate votes which may complicate things.


#### An example of a possible implementation path

`LightClientAttackEvidence` won't change but the evidence interface will need to look like the proposed one above and `DuplicateVoteEvidence` will need to change to encompass multiple double votes. A problem with batch evidence is that it needs to be unique to avoid people from submitting different permutations.

> I most likely have not captured the full picture of all these approaches and therefore would encourage others to add their takes on each of them.


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

The 50% approach would be such: If a validator received a proposal that didn't contain at least one of the evidence that that validator had in its own evidence pool it would consider the proposal invalid and thus would vote `nil`. This solution is far less intrusive to the current consensus algorithm but it also doesn't quite solve the problem of censorship, rather this protocol will keep the chain halted until off-chain governance can intervene.

The 100% approach is a lot more intricate and would be to find a protocol where upon receiving a valid proposal with evidence, current validators that had been shown to have misbehaved would be barred from voting such that the total voting power would be reduced by the voting power of the malicious validators and this would allow for 2/3 consensus to be reached regardless of what the malicious validators did.


## Proposal

A Hybrid design

Evidence has the following simple interface:

```go
type Evidence interface {  //proposed
  Height() int64                                     // height of the equivocation
  Bytes() []byte                                     // bytes which comprise the evidence
  Hash() []byte                                      // hash of the evidence
  ValidateBasic() error
  String() string
}
```

We have two concrete types of evidence that fulfil this interface

```go
type LightClientAttackEvidence struct {
  ConflictingBlock *LightBlock
  CommonHeight int64
}
```
where the `Hash()` is the hash of the header and commonHeight

```go
type DuplicateVoteEvidence {
  VoteA *Vote
  VoteB *Vote
}
```
where the `Hash()` is the hash of the two votes

The first is generated in the light client, the second in consensus. Both are sent to the evidence pool through `AddEvidence(ev Evidence) error` where it first runs `Has(ev Evidence)` to check if it has already received it (by comparing hashes) and then  `Verify(ev Evidence) error` before adding it to the database. There are two databases: one
for pending evidence that is not yet committed and another of the committed evidence (on the chain)


`Verify()` does the following:

- Use the hash to see if we already have this evidence in our committed database.

- Use the height to check if the evidence hasn't expired.

- If it hasn't expired use the height to find the block header and confirm that the time hasn't also expired.

- Then proceed with switch statement for each of the two evidence:

For `DuplicateVote`:

- Check that height, round, type and validator address are the same

- Check that the Block ID is different

- Check the look up table for addresses to make sure there already isn't evidence against this validator

- Fetch the validator set and confirm that the address is in the set at the height of the attack

- Check that the chain ID and signature is valid.

For `LightClientAttack`

- Fetch the common signed header and val set from the common height and use skipping verification to verify the conflicting header

- Fetch the trusted signed header at the same height as the conflicting header and compare with the conflicting header to work out which type of attack it is and in doing so return the malicious validators.

  - If equivocation the validators that signed for the commits of both the trusted and signed header

  - If lunatic the validators from the common val set that signed in the conflicting block

  - If amnesia none (we can't know which validators are malicious). This also means that we don't
  send amnesia evidence to the application

- For each validator, check the look up table to make sure there already isn't evidence against this validator


After verification we save the evidence with the following key `height/hash` and in the following struct

```go
type EvidenceInfo struct {
  ev Evidence
  time time.Time
  validators []Validator
  totalVotingPower int64
}
```

We save extra information as this will be useful in forming the ABCI Evidence that we will come to later

Receiving broadcasts from other evidence reactors works in the same manner.

When it comes to prevoting and precomitting a proposal that contains evidence, the full node will once again
call upon the evidence pool to verify the evidence using `CheckEvidence(ev []Evidence)`:

This loops through all the evidence first checking that nothing has been duplicated then:

runs `fastCheck(ev evidence)` which works similar to `Has` but with `LightClientAttackEvidence` if it has the
same hash it then checks to see that the validators it has are all signers in the commit of the conflicting header. If it doesn't pass fast check (because it hasn't seen the evidence before) then it will have to verify the evidence.

runs `Verify(ev Evidence)` - Note: this also saves the evidence to the db as mentioned before.


The final part of the lifecycle is when the block is committed and the `BlockExecutor` tries to apply state. Instead of trying to form the information necessary to produce the ABCI evidence it gets the evpool to create the evidence in the update step as follows.

`Update(Block, State) []abci.Evidence`

```go
abciResponses.BeginBlock.ByzantineValidators = evpool.Update(block, state)
```

Update does the following:
- increments state which keeps track of both the current time and height used for measuring expiry
- marks evidence as committed and saves to db. This prevents validators from proposing committed evidence in the future
  Note: the db just saves the height and the hash. There is no need to save the entire committed evidence
- forms abci evidence:
```go
for _, val := range evInfo.Validators
  abciEv = append(abciEv, &abci.Evidence{
    Type: evType,
    Validator: val,
    Height: evInfo.ev.Height(),
    Time: evInfo.time,
    TotalVotingPower: evInfo.totalVotingPower
  })

```
and compiles it all together (note for `DuplicateVoteEvidence` the validators array size is 1)
- removes expired evidence from both pending and committed


## Status

Proposed

## Consequences

<!-- > This section describes the consequences, after applying the decision. All consequences should be summarized here, not just the "positive" ones. -->

### Positive

- Evidence is better contained to the evidence pool / module
- LightClientAttack is kept together (easier for verification and bandwidth)
- Variations on commit sigs in LightClientAttack doesn't lead to multiple permutations and multiple evidence
- Address to evidence map prevents dos attacks of evidence by a single validator

### Negative

- Breaking change to Evidence interface and thus evidence
- Unable to query evidence for address / time without evidence pool

### Neutral

- Doesn't break the ABCI Evidence struct

## References

<!-- > Are there any relevant PR comments, issues that led up to this, or articles referenced for why we made the given design choice? If so link them here! -->

- [LightClientAttackEvidence](https://github.com/informalsystems/tendermint-rs/blob/31ca3e64ce90786c1734caf186e30595832297a4/docs/spec/lightclient/attacks/evidence-handling.md)
