# ADR 018: ABCI Validator Improvements

## Changelog

05-08-2018: Initial draft

## Context

ADR 009 introduced major improvements to the ABCI around validators and the use
of Amino. Here we follow up with some additional changes to improve the naming
and expected use of Validator messages.

We also fix how we communicate the commit round - there is no defined commit
round, as validators can commit the same block in different rounds, so we
should communicate the round each validator committed in.

## Decision

### Validator

Currently a Validator contains address and `pub_key`, and one or the other is
optional/not-sent depending on the use case. Instead, we should have a
Validator (with just the address) and a ValidatorUpdate (with the pubkey):

```
message Validator {
    bytes address
    int64 power
}

message ValidatorUpdate {
    PubKey pub_key
    int64 power
}
```

### RequestBeginBlock

LastCommitInfo currently has an array of `SigningValidator` that contains
information for each validator in the entire validator set.
Instead, this should be called `VoteInfo`, since it is information about the
validator votes.

Additionally, we have a single CommitRound in the LastCommitInfo,
but such a round does not exist. Instead, we
should include the round associated with each commit vote:

```
message LastCommitInfo {
  repeated VoteInfo commit_votes
}

message VoteInfo {
    Validator validator
    bool signed_last_block
    int64 round
}
```

### ResponseEndBlock

Use ValidatorUpdates instead of Validators. Then it's clear we don't need an
address, and we do need a pubkey.

### InitChain

Use ValidatorUpdates for both Request and Response. InitChain
is about setting/updating the initial validator set, unlike BeginBlock
which is just informational.

## Status

Proposal.

## Consequences

### Positive

- Easier for developers to build on and understand the ABCI
- Apps get more information about the votes (ie. the round they're from)

### Negative

- There are two validator types

### Neutral

-

## References

- [Latest ABCI Spec](https://github.com/tendermint/tendermint/blob/v0.22.8/docs/app-dev/abci-spec.md)
- [ADR-009](https://github.com/tendermint/tendermint/blob/v0.22.8/docs/architecture/adr-009-ABCI-design.md)
- [Issue #1712 - Don't send PubKey in
  RequestBeginBlock](https://github.com/tendermint/tendermint/issues/1712)
