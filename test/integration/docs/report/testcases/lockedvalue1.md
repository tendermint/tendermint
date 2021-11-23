# Testing Unlocking of Proposal

Testcase - https://github.com/ds-test-framework/tendermint-test/tree/master/testcases/lockedvalue/one.go

## Goal

The goal is to test if a replica, after locking on a proposal, unlocks when it sees a majority vote for nil.
This is a behaviour that deviates from the protocol description.

We ensure that a replica locks onto a proposal, unlocks it when it sees a majority of nil votes and then prevotes on the new proposal

## Method

We partition the set of `n=3f+1` replicas into `1,f, 2f` partitions with labels `h,faulty,rest`. Similar to roundskips, in round 0

1. We delay the `Prevote` messages originating from `h` indefinitely
2. Change the `Prevote` messages originating from `faulty` to nil.

Then in round 1, we do not deliver the proposal.

## Working

In round 0, all honest replicas except `h` fail to see a consensus of `Prevote`s and hence will not lock onto the proposal. `h` however will lock the proposal.

In round 1, when `h` sees `2f+1` nil `Prevote`s, it unlocks the locked value and prevotes the proposal of round 2.

## Outcome

1. We observe that `h` prevotes on the locked proposal in round 1.
2. We observe that `h` relocks and prevotes on the proposal of round 2.

## Dual scenario

Testcase - https://github.com/ds-test-framework/tendermint-test/tree/master/testcases/lockedvalue/two.go

We also created a testcase where we test for the dual condition. Specifically, we expect that `h` does not unlock in round 1 and continues to prevote on the locked block. This is the expected behaviour as per the protocol spec.
