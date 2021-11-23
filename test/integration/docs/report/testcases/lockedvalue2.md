# Testing relocking

Testcase - https://github.com/ds-test-framework/tendermint-test/tree/master/testcases/lockedvalue/three.go

## Goal

We test that a replica which has previously locked onto a particular proposal is able to relock onto a new proposal if it sees a quorum.

## Method

We partition the set of `n=3f+1` replicas into `1,f, 2f` partitions with labels `h,faulty,rest`. Similar to roundskips, in round 0

1. We delay the `Prevote` messages originating from `h` indefinitely
2. Change the `Prevote` messages originating from `faulty` to nil.

Then, from round 1 onwards we do not deliver a proposal until is different (`new_proposal`) from the one in round 0 (`old_proposal`). Once we see a different proposal, we change the `Prevote`s of `faulty` to the new proposal only when the vote is sent to `h`. We now expect `h` to precommit on `new_proposal`.

The `Precommit` messages orignating from `faulty` will always be changed to nil.

## Working

Here, we force `h` to lock onto a proposal (`old_proposal`) as it is the only replica which sees a consensus in round 0. Subsequently, when we sees a proposal different (`new_proposal`) than the one in round 0, we ensure that `h` sees a consensus for that proposal. Here, `h` should change its locked value to `new_proposal` and hence precommit on it.

## Outcome

1. We observe that `h` locks onto the `old_proposal` and prevotes it.
2. We also observe that `h` precommits `new_proposal` once it sees a consensus of `Prevote`s.
