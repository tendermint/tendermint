# ADR 056: Proving amnesia attacks

## Changelog

- 02.04.20: Initial Draft
- 06.04.20: Second Draft
- 10.06.20: Post Implementation Revision
- 19.08.20: Short Term Amnesia Alteration

## Context

Whilst most created evidence of malicious behaviour is self evident such that any individual can verify them independently there are types of evidence, known collectively as global evidence, that require further collaboration from the network in order to accumulate enough information to create evidence that is individually verifiable and can therefore be processed through consensus. [Fork Accountability](https://github.com/tendermint/spec/blob/master/spec/consensus/light-client/accountability.md) has been coined to describe the entire process of detection, proving and punishing of malicious behaviour. This ADR addresses specifically how to prove an amnesia attack but also generally outlines how global evidence can be converted to individual evidence.

### Amnesia Attack

The currently only known form of global evidence stems from [flip flopping](https://github.com/tendermint/spec/blob/master/spec/consensus/light-client/accountability.md#flip-flopping) attacks. The schematic below explains one scenario where an amnesia attack, a form of flip flopping, can occur such that two sets of honest nodes, C1 and C2, commit different blocks.

![](../imgs/tm-amnesia-attack.png)

1. C1 and F send PREVOTE messages for block A.
2. C1 sends PRECOMMIT for round 1 for block A.
3. A new round is started, C2 and F send PREVOTE messages for a different block B.
4. C2 and F then send PRECOMMIT messages for block B.
5. F breaks the lock and goes back and sends PRECOMMIT messages in round 1 for block A.


This creates a fork on the main chain.  Back to the past, another form of flip flopping, creates a light fork (capable of fooling those not involved in consensus). This is done in a similar fashion to the schematic above, however the validators C1 eventually progress and commit the block in Round 2 and then when a light client comes to validate the block at that height, the nodes take the precommits for Round 1 and forge their own precommits to produce what looks like a valid block for the light client.


## Pretext

An amnesia protocol was outlined in a previous revision of this ADR and for completeness has been appeneded at the bottom (Appendix A). However, under the circumstances of the impending IBC release it was adjudged that the protocol hadn't received enough rigour and could be potentially liable for opening other forms of misbehaviour especially considering that the nature of the protocol deemed all the nodes that committed for a certain block guilty until they proved their innocence. 

## Decision

The decision surrounding amnesia attacks has both a short term and long term component. In the long term, a more sturdy protocol will need to be fleshed out and implemented. There is already draft documents outlining what such a protocol would look like and the resources it would require. In the short term, it was discussed whether the protocol should be completely removed or if there should remain some logic in handling the aforementioned scenarios.

The latter of the two options was decided chiefly because it is important for the tendermint incentivisation mechanism that such behavior towards a light client is not only detectable but punishable. The logic that will need to be in place will involve the bare minimum to enable manual intervention. This therefore requires the on-chain submission of the faulty header that the light client witnessed plus the storing of vote sets by validators.

## Detailed Design

The first part of this short term solution is the `PotentialAmnesiaEvidence` data structure. This has the following properties that differ from its prior implementation.

- It bundles all the malicious validators together instead of splitting them into individual evidence
- It can be submitted on the chain
- It does, by itself, not indicate which validators in the set misbehaved and should be slashed. It is merely a prompt for manual investigation.

Before going any further, it is also important to mention that `PotentialAmnesiaEvidence` should only be formed from a valid attack on the light client and measures should be put in place to ensure that no node can easily forge the evidence as a means of spamming the network. 

The data structure for `PotentialAmensiaEvidence` is as follows:

```golang
type PotentialAmnesiaEvidence struct {
	*SignedHeader
}
```

It conforms to the `Evidence` interface where `Time()` is the time of the heaer (note that this can be forged but must be within trusting period) and `Address()` returns the bytes of all validators in the commit

`ValidateBasic()` inherit from the validate basic of the signed header

### Verification of PotentialAmensiaEvidence

`PotentialAmensiaEvidence` will be saved in the evidence pool and committed on chain if the following conditions are met.

- The `SignedHeader` is valid -> `ValidateBasic()`

- The `ValidatorsHash` of the header must be the same as the header that the node has committed. (else this is a lunatic attack)

- The signatures of the commit must all be valid (for that header) `VerifyCommit()` using the validator set the node has for that height.

- The header hash must be different to the hash of the header that the node has.

- The `Commit` of the `SignedHeader` must be for a different round than the `Commit` of the header that the node has committed

- The evidence must not have expired.


The second part of the short term solution is saving `VoteSet`'s to the evidence pool. In order to avoid overflowing the validators memory, only the relevant information will be taken from the voteSets and formed into
this specific struct

```golang
type VotesRecord struct {
	votes []*Vote
}
```

These votes should all be precommit votes of the same height and round and will come from the consensus reactor. `VotesRecord` will only be created in heights where there are more than 1 round and will be sent from consensus. 
`VotesRecord` will also follow the same pruning algorithm as the rest of the evidence, being removed after expiring. 

Implementing this is in the short term should be sufficient to detecting amnesia attacks using manual intervention which through off-chain conesnsus can lead to punishment and thus in itself act to disincentivise misbehaviour.

## Status

Proposed

## Consequences

### Positive

Increasing fork detection and accountability makes the system more secure

### Negative

Non-responsive but honest nodes that are part of the suspect group that don't produce a proof will be punished

A delay between the detection of a fork and the punishment of one

### Neutral


## References

- [Fork accountability algorithm](https://docs.google.com/document/d/11ZhMsCj3y7zIZz4udO9l25xqb0kl7gmWqNpGVRzOeyY/edit)
- [Fork accountability spec](https://github.com/tendermint/spec/blob/master/spec/consensus/light-client/accountability.md)

## Appendix A: Prior AmnesiaEvidence Implementation

As the distinction between these two attacks (amnesia and back to the past) can only be distinguished by confirming with all validators (to see if it is a full fork or a light fork), for the purpose of simplicity, these attacks will be treated as the same.

Currently, the evidence reactor is used to simply broadcast and store evidence. The idea of creating a new reactor for the specific task of verifying these attacks was briefly discussed, but it is decided that the current evidence reactor will be extended.

The process begins with a light client receiving conflicting headers (in the future this could also be a full node during fast sync or state sync), which it sends to a full node to analyse. As part of [evidence handling](https://github.com/tendermint/tendermint/blob/master/docs/architecture/adr-047-handling-evidence-from-light-client.md), this is extracted into potential amnesia evidence when the validator voted in more than one round for a different block.

```golang
type PotentialAmnesiaEvidence struct {
	VoteA *types.Vote
	VoteB *types.Vote

	Heightstamp int64
}
```

*NOTE: There had been an earlier notion towards batching evidence against the entire set of validators all together but this has given way to individual processing predominantly to maintain consistency with the other forms of evidence. A more extensive breakdown can be found [here](https://github.com/tendermint/tendermint/issues/4729)*

The evidence will contain the precommit votes for a validator that voted for both rounds. If the validator voted in more than two rounds, then they will have multiple `PotentialAmnesiaEvidence` against them hence it is possible that there is multiple evidence for a validator in a single height but not for a single round. The votes should be all valid and the height and time that the infringement was made should be within:

`MaxEvidenceAge - ProofTrialPeriod`

This trial period will be discussed later.

Returning to the event of an amnesia attack, if we were to examine the behaviour of the honest nodes, C1 and C2, in the schematic, C2 will not PRECOMMIT an earlier round, but it is likely, if a node in C1 were to receive +2/3 PREVOTE's or PRECOMMIT's for a higher round, that it would remove the lock and PREVOTE and PRECOMMIT for the later round. Therefore, unfortunately it is not a case of simply punishing all nodes that have double voted in the `PotentialAmnesiaEvidence`.

Instead we use the Proof of Lock Change (PoLC) referred to in the [consensus spec](https://github.com/tendermint/spec/blob/master/spec/consensus/consensus.md#terms). When an honest node votes again for a different block in a later round
(which will only occur in very rare cases), it will generate the PoLC and store it in the evidence reactor for a time equal to the `MaxEvidenceAge`

```golang
type ProofOfLockChange struct {
	Votes []*types.Vote
	PubKey crypto.PubKey
}
```

This can be either evidence of +2/3 PREVOTES or PRECOMMITS (either warrants the honest node the right to vote) and is valid, among other checks, so long as the PRECOMMIT vote of the node in V2 came after all the votes in the `ProofOfLockChange` i.e. it received +2/3 votes for a block and then voted for that block thereafter (F is unable to prove this).

In the event that an honest node receives `PotentialAmnesiaEvidence` it will first `ValidateBasic()` and `Verify()` it and then will check if it is among the suspected nodes in the evidence. If so, it will retrieve the `ProofOfLockChange` and combine it with `PotentialAmensiaEvidence` to form `AmensiaEvidence`. All honest nodes that are part of the indicted group will have a time, measured in blocks, equal to `ProofTrialPeriod`, the aforementioned evidence paramter, to gossip their `AmnesiaEvidence` with their `ProofOfLockChange`

```golang
type AmnesiaEvidence struct {
	*types.PotentialAmnesiaEvidence
	Polc   *types.ProofOfLockChange
}
```

If the node is not required to submit any proof than it will simply broadcast the `PotentialAmnesiaEvidence`, stamp the height that it received the evidence and begin to wait out the trial period. It will ignore other `PotentialAmnesiaEvidence` gossiped at the same height and round.

If a node receives `AmnesiaEvidence` that contains a valid `ProofOfClockChange` it will add it to the evidence store and replace any PotentialAmnesiaEvidence of the same height and round. At this stage, an amnesia evidence with polc, it is ready to be submitted to the chin. If a node receives `AmnesiaEvidence` with an empty polc it will ignore it as each honest node will conduct their own trial period to be sure that time was given for any other honest nodes to respond.

There can only be one `AmnesiaEvidence` and one `PotentialAmneisaEvidence` stored for each attack (i.e. for each height).

When, `state.LastBlockHeight > PotentialAmnesiaEvidence.timestamp + ProofTrialPeriod`, nodes will upgrade the corresponding `PotentialAmnesiaEvidence` and attach an empty `ProofOfLockChange`. Then honest validators of the current validator set can begin proposing the block that contains the `AmnesiaEvidence`.

*NOTE: Even before the evidence is proposed and committed, the off-chain process of gossiping valid evidence could be
 enough for honest nodes to recognize the fork and halt.*

Other validators will vote <nil> if:

- The Amnesia Evidence is not valid
- The Amensia Evidence is not within their own trial period i.e. too soon.
- They don't have the Amnesia Evidence and it is has an empty polc (each validator needs to run their own trial period of the evidence)
- Is of an AmnesiaEvidence that has already been committed to the chain.

Finally it is important to stress that the protocol of having a trial period addresses attacks where a validator voted again for a different block at a later round and time. In the event, however, that the validator voted for an earlier round after voting for a later round i.e. `VoteA.Timestamp < VoteB.Timestamp && VoteA.Round > VoteB.Round` then this action is inexcusable and can be punished immediately without the need of a trial period. In this case, PotentialAmnesiaEvidence will be instantly upgraded to AmnesiaEvidence. 