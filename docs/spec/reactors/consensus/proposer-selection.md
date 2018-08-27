# Proposer selection procedure in Tendermint

This document specifies the Proposer Selection Procedure that is used in Tendermint to choose a round proposer.
As Tendermint is “leader-based protocol”, the proposer selection is critical for its correct functioning.
Let denote with `proposer_p(h,r)` a process returned by the Proposer Selection Procedure at the process p, at height h
and round r. Then the Proposer Selection procedure should fulfill the following properties:

`Agreement`: Given a validator set V, and two honest validators,
p and q, for each height h, and each round r,
proposer_p(h,r) = proposer_q(h,r)

`Liveness`: In every consecutive sequence of rounds of size K (K is system parameter), at least a
single round has an honest proposer.

`Fairness`: The proposer selection is proportional to the validator voting power, i.e., a validator with more
voting power is selected more frequently, proportional to its power. More precisely, given a set of processes
with the total voting power N, during a sequence of rounds of size N, every process is proposer in a number of rounds
equal to its voting power.

We now look at a few particular cases to understand better how fairness should be implemented.
If we have 4 processes with the following voting power distribution (p0,4), (p1, 2), (p2, 2), (p3, 2) at some round r,  
we have the following sequence of proposer selections in the following rounds:

`p0, p1, p2, p3, p0, p0, p1, p2, p3, p0, p0, p1, p2, p3, p0, p0, p1, p2, p3, p0, etc`

Let consider now the following scenario where a total voting power of faulty processes is aggregated in a single process
p0: (p0,3), (p1, 1), (p2, 1), (p3, 1), (p4, 1), (p5, 1), (p6, 1), (p7, 1).
In this case the sequence of proposer selections looks like this:

`p0, p1, p2, p3, p0, p4, p5, p6, p7, p0, p0, p1, p2, p3, p0, p4, p5, p6, p7, p0, etc`

In this case, we see that a number of rounds coordinated by a faulty process is proportional to its voting power.
We consider also the case where we have voting power uniformly distributed among processes, i.e., we have 10 processes
each with voting power of 1. And let consider that there are 3 faulty processes with consecutive addresses,
for example the first 3 processes are faulty. Then the sequence looks like this:

`p0, p1, p2, p3, p4, p5, p6, p7, p8, p9, p0, p1, p2, p3, p4, p5, p6, p7, p8, p9, etc`

In this case, we have 3 consecutive rounds with a faulty proposer.
One special case we consider is the case where a single honest process p0 has most of the voting power, for example:
(p0,100), (p1, 2), (p2, 3), (p3, 4). Then the sequence of proposer selection looks like this:

p0, p0, p0, p0, p0, p0, p0, p0, p0, p0, p0, p0, p0, p1, p0, p0, p0, p0, p0, etc

This basically means that almost all rounds have the same proposer. But in this case, the process p0 has anyway enough
voting power to decide whatever he wants, so the fact that he coordinates almost all rounds seems correct.
