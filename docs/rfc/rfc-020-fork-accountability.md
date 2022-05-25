# RFC 020: Fork Accountability

## Changelog

- XX-April-2022: Initial draft (@cason).

## Abstract

This document discusses methods for holding validator nodes accountable for
their misbehavior in the consensus protocol. 
We restrict the discussion to two forms of misbehavior: **double voting** and
**amnesia**.
It has been proved that for there to be a violation of the agreement property
of consensus, which may lead to forks in the blockchain, one of these two forms
of misbehavior must have occurred.
We discuss practical aspects and existing limitations for detecting and
producing irrefutable evidence against validators that performed these attacks.

## Background

### Consensus basics

Tendermint’s consensus [protocol][arxiv] runs a sequence of instances of
consensus, where each instance decides on a block of transactions. 
Each instance or *height* of consensus is composed of one or more *rounds* of
consensus, always starting from round 0.
Each round of consensus is led by a validator, the round’s *proposer*, and
is composed of three round steps: *propose*, *prevote*, and *precommit*.
In the *propose* step, nodes wait for the block proposed by the round's
proposer; a validator can either accept or reject the proposed block.
The *prevote* and *precommit* steps are voting steps, where validators are
expected to sign and broadcast votes.
A vote contains a height, a round, a type (`Prevote` or `Precommit`, depending
on the round step), and a value, which is either a `BlockID` (the unique
identifier of a block) or `nil` ("no block").
The signature attached to a valid vote allows the unequivocal identification of
the validator that cast the vote.

Voting steps succeed when a node receives identical votes (i.e., for the same
value) from validators whose aggregated voting power is larger than 2/3 of the
total voting power (of all validators).
We use `2/3+` to denote any set of validators, or votes cast by validators,
that attend to the above condition.
So, if a validator receives `2/3+ Prevotes` for a value in the *prevote* step,
the validator casts a `Precommit` for the value.
If the value is not `nil`, the validator *locks* the value before casting a
`Precommit` for it.
Moreover, if a node receives `2/3+ Precommits` for a value that is not `nil`,
the node *decides* the block uniquely identified by that value (which is a
`BlockID`), concluding its execution on that height of consensus.

### Byzantine (mis)behavior

We distinguish consensus participants (validators, mainly) between *correct*,
when they strictly follow the algorithm, and *Byzantine*, that can arbitrarily
deviate from the algorithm.
The protocol assumes that up to `f` nodes are Byzantine and requires a total of
`3f+1` nodes (i.e., `2f+1` correct nodes) for a safe operation.
Considering that nodes may have distinct voting powers, this means that `2/3+`
validators (nodes with positive voting power) must be correct.

Byzantine nodes may misbehave in multiple ways.
In this document, we restrict our analysis to two forms of misbehavior.
Both attacks consist on actions taken by validator nodes that deviate from the
algorithm when casting votes.

#### Double voting

A validator is expected to cast a single vote per round step, *prevote* or
*precommit*.
A Byzantine validator may cast conflicting votes (for distinct values) in the
same round and round step.
The goal is to deceive correct validators, as some of them may receive (and
take actions based on) the Byzantine validator's vote for a value, while others
will assume that the validator voted for a different value.

Double voting (also known as *equivocation*) attack is on the Byzantine
behavior's playbook, and is the more effective the simpler it is to direct
votes (and messages in general) to specific recipients.
As Tendermint relies on gossip-based communication, the effectiveness of this
attack is somehow restricted, as it is not trivial for a Byzantine node to
restrict the set of nodes that will receive a specific message.
Moreover, if a node receives two conflicting votes signed by a validator, it
can produce an irrefutable evidence of misbehavior against it.

#### Amnesia

The algorithm establishes that a validator must *lock* a value before casting a
`Precommit` vote for it; this does not apply to `nil` values.
The existence of a locked value determines the operation of the validator on
the *prevote* step of subsequent rounds.
In short, a validator cannot cast a `Prevote` for a non-`nil` value if it is
locked on a different value.
This is a mechanism adopted by several consensus algorithms to ensure that
different rounds do not end up deciding distinct values.

A Byzantine validator can cast votes for any value, in particular it can simply
ignore the locking mechanism.
What makes this attack relevant, however, is the existence of legit execution
scenarios in which a correct validator can cast a `Precommit` for a value,
then, in a subsequent round, cast a `Prevote` for a different value.
This happens because, unlike most consensus algorithms, Tendermint allows,
under specific circumstances, nodes to *unlock* a value in order to lock on
another value.
As a result, it is not straightforward to produce irrefutable evidence
demonstrating that a validator has committed an amnesia attack.

> This observation considers the current specification and implementation of
> the consensus algorithm.
> A relatively small set of [changes][amnesia] in the consensus algorithm was
> proposed so that to enable the accurate detection of amnesia attacks.
> This proposal and its impact on the consensus implementation are discussed in
> the remaining of this document.

### References

[arxiv]: https://arxiv.org/abs/1807.04938

> Links to external materials needed to follow the discussion may be added here.
>
> In addition, if the discussion in a request for comments leads to any design
> decisions, it may be helpful to add links to the ADR documents here after the
> discussion has settled.

## Discussion

> This section contains the core of the discussion.
>
> There is no fixed format for this section, but ideally changes to this
> section should be updated before merging to reflect any discussion that took
> place on the PR that made those changes.
