# Round skips using BlockParts

## Goal

We expect that replicas skip to the next round when they do not receive the complete set of BlockPart messages that make up a Block.

## Method

We identify from the set of `n=3f+1` replicas, `f+1` replicas other than the Proposer. We then do not deliver `BlockPart` messages to these replicas.

## Working

The replicas do not receive the complete `ProposalBlock` and `Prevote` nil. This will lead to replicas not seeing enough `Prevote`s on the `ProposedBlock` and hence `Precommit` nil. This leads to a round change.

The part of the code that we are testing is what happens when you do not receive a valid proposal. Here, tendermint considers a `Proposal` valid when it has the complete block. Also, the parts of the code which checks if the votes are counted correctly

## Outcome

1. We observe that replicas that do not receive the complete block send nil `Prevote` messages.
2. We observe that replicas do not see a consensus in the `Prevote` step and `Precommit` nil.
3. Crucially, we observe that the replicas move to the next round.
4. Also, it is important to note that when we want to do multiple round skips we need to make sure the proposer is not one of the `f+1` replicas that we picked.
