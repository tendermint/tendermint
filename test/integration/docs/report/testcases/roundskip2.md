# Round skip using `Prevote`

Testcase - https://github.com/ds-test-framework/tendermint-test/tree/master/testcases/rskip/one.go

## Goal

We extect the replicas to skip rounds when they do not see enough `Prevote` messages from other replicas. Furthermore, we would like to observe when delayed `Prevote` messages arrive at the replica.

## Method

We partition the set of `n=3f+1` replicas into `1, f, 2f` and label the partitions as `h, faulty, rest`. We ensure that all replicas except `h` do not see enough `Prevote` messages to achieve consensus.

In each round,

1. We delay the `Prevote` message of `h` indefinitely (until we are done skipping rounds)
2. We change the `Prevote` messsage contents of `faulty` replicas to nil.

## Working

All honest replicas except `h` do not see a consensus of votes for the proposal.

The replicas in the `rest` partition will receive `2f Prevote` messages from all members in the `rest` partition. Also, they will receive `f` nil `Prevote` messages from `faulty` replicas.

The `faulty` replica's votes are changes and hence we can consider them to be byzantine.

Hence, none of the replicas in `rest` `Precommit` on the proposal. This will lead to a round skip as no replica will see a consensus of `Precommit`s

## Outcome

1. We observe that the replicas in the `rest` partition necessarily `Precommit` on nil.
2. The replicas move to the next round. This processes can be repeated to any number of rounds.
3. If we deliver the delayed votes of `h`, the replicas will commit when `h` proposes a block.
4. More crucially, we observe that `h` proposes that block for which it last saw a consensus of `Prevote` messages. (`LockedValue`)
