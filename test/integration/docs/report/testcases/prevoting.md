# Prevote a proposal with higher valid round

Testcase - Testcase - https://github.com/ds-test-framework/tendermint-test/tree/master/testcases/sanity/higherProp.go

## Goal

We are testing a scenario where a replica has,

- locked onto a proposal with `l` as its locked round
- receives a proposal with valid round `v_r > l`
- has seen a consensus of `Prevote`s in round `v_r`

And here, we expect the replica to `Prevote` the new proposal.

## Method

We partition the set of `n=3f+1` replicas into `1,f, 2f` partitions with labels `h,faulty,rest`. Similar to roundskips, in round 0

1. We delay the `Prevote` messages originating from `h` indefinitely
2. Change the `Prevote` & `Precommit` messages originating from `faulty` to nil.
3. We record the proposal as `old_proposal`

From round 1 onwards, we do not deliver proposal messages unless the proposal is different from `old_proposal`. When we have a new proposal, we

1. Record the proposal as `new_proposal`
2. Change the `Prevote` originating from `faulty` to `new_proposal` if they are intended to `rest`

## Working

In round 0 we ensure that `h` has locked onto `old_proposal`. Thereby setting `l=0`. When we see `new_proposal`, we ensure that all replicas in the partition `rest` see consensus of votes and hence lock on `new_proposal`. We wait until `new_proposal` is reproposed.

We can expect this since `rest` does not see a consensus of `Precommit` votes in order to commit.

Once `new_proposal` is reproposed, it will contain `v_r > l`. Here, we expect that `h` prevotes `new_proposal` since it contains a `v_r > l`. 

## Outcomes

We do not observe the expected behaviour. The replica `h` continues to prevote its locked block irrespective of any other condition. (We are assuming that the replica does not unlock).

This is a deviation of the implementation from the protocol specification.
