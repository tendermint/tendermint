# Proposer selection procedure in Tendermint

This document specifies the Proposer Selection Procedure that is used in Tendermint to choose a round proposer.
As Tendermint is “leader-based protocol”, the proposer selection is critical for its correct functioning.

At a given block height, the proposer selection algorithm runs with the same validator set at each round .
Between heights, an updated validator set may be specified by the application as part of the ABCIResponses' EndBlock. 

## Requirements for Proposer Selection

This sections covers the requirements with Rx being mandatory and Ox optional requirements.
The following requirements must be met by the Proposer Selection procedure:

#### R1: Determinism
Given a validator set `V`, and two honest validators `p` and `q`, for each height `h` and each round `r` the following must hold:

  `proposer_p(h,r) = proposer_q(h,r)`

where `proposer_p(h,r)` is the proposer returned by the Proposer Selection Procedure at process `p`, at height `h` and round `r`.

#### R2: Liveliness
In every consecutive sequence of rounds of size K (K is system parameter), at least a
single round has an honest proposer (??)

#### R3: Fairness
In a stable network (no validator set changes) and for any sequence of K rounds with K > P, a validator v is elected as proposer with a frequency f proportional to its voting power VP(v) divided by the total voting power P:

    f ~ VP(v) / P

#### O4: Starvation Prevention ;)
In a churning network with many validator set changes, minimal guarantees for a chance in future election should be offered, e.g. for stable lower power validators.

## Proposer Selection Specification
### Requirement Fulfillment Claims
__[R1]__ The proposer algorithm is deterministic giving consistent results across executions with same transactions and validator set modifications. A pseudocode of the alorithm is given below. 

__[R2]__ ??

__[R3]__ Given a set of processes with the total voting power P, during a sequence of rounds of length P, every process is selected as proposer in a number of rounds equal to its voting power. The sequence of the P proposers then repeats.
If we consider the validator set:
Validator | p1 | p2 
----------|--- | ---
VP        | 1  | 3

The proposer selection generates the sequence:
`p2, p1, p2, p2, p2, p1, p2, p2,...` or [`p2, p1, p2, p2`]*

Assigning priorities to each validator based on the voting power and updating them at each round ensures the fairness of the proposer selection. In addition, every time a validator is elected as proposer its priority is decreased with the total voting power.

__[O4]__ There is currently no definite guarantee that a low power stable validator will be elected within a large enough round window (??)
Note: Just a claim here. If new high power validators bound and unbound continuously, the low validator moves back and forward in the queue via shifting. Since there is no "aging" that would cause a steady increase in its voting power, or similar mechanism, it is possible that this validator election is postponed indefinitely.

### Basic Algorithm

This section includes an overview of the proposer selection algorithm. 

A model that gives a good intuition on how/ why the selection algorithm works and it is fair is that of a priority queue. The validators move ahead in this queue according to their voting power (the higher the voting power the faster a validator moves towards the head of the queue). At each round the following happens:
- all validators move "ahead" according to their powers: for each validator, increase the priority by the voting power
- first in the queue becomes the proposer: select the validator with highest priority 
- move the proposer to the end of the queue: decrease the proposer's priority by the total voting power

Notation:
- vset - the validator set
- A(i) - accumulated priority for validator i
- VP(i) - voting power of validator i
- P - total voting power of set
- avg - average of all validator priorities
- prop - proposer

Simple view at the Selection Algorithm:

```
for each validator i in set:
  A(i) += VP(i)
prop = max(A)
A(prop) -= P
```

If the set of the validators has not changed, at the beginning of each new round the sum of the proposal priorities is 0.

Following table shows, for the example above, how the proposer priority is calculated during each round. Only 4 rounds are shown, starting with the 5th the same values are computed.
Each row shows the imaginary queue and the process place in it. The proposer is the closest to the head of the queue, the rightmost validator. As priorities are updated, the validators move right in the queue. At the end of the round the proposer moves left as its priority is reduced after election. At the end of each round the sum of the priorities is 0.

Priority Round  | -2 | -1 | 0    | 1   | 2   | 3 | 4 | 5 | Alg step
--------------- | ---|--- |------|---  |---  |---|---|---|--------
 |              |    |    |p1,p2 |     |     |   |   |   |Initialized to 0
 |1             |    |    |      |  p1 |     | p2|   |   |A(i)+=VP(i)
 |              |    | p2 |      |  p1 |     |   |   |   |A(p2)-= P
 |2             |    |    |      |     |p1,p2|   |   |   |A(i)+=VP(i)
 |              | p1 |    |      |     |  p2 |   |   |   |A(p1)-= P
 |3             |    | p1 |      |     |     |   |   | p2|A(i)+=VP(i)
 |              |    | p1 |      | p2  |     |   |   |   |A(p2)-= P
 |4             |    |    |   p1 |     |     |   | p2|   |A(i)+=VP(i)
 |              |    |    |p1,p2 |     |     |   |   |   |A(p2)-= P

The actual implementation does not create this queue, it only has to store the current accumulated proposer priority. 

### Validator Set Changes
At each block height the validator set may change. Some of the changes have implications on the proposer selection.

#### Voting Power Change
Consider again the earlier example and assume that the voting power of p1 is changed to 6:
Validator | p1 | p2 
----------|--- | ---
VP        | 4  | 3

Let's also assume that the last round R before this change the proposer priorites were as shown in first row (R-1 end). As it can be seen, the procedure can continue with round 1 at next height without changes as before. 

|Priority Round| -2 | -1 | 0    | 1   | 2   | 3 | 4 | 5 | Comment
|--------------| ---|--- |------|---  |---  |---|---|---|--------
| R end        |    | p2 |      |  p1 |     |   |   |   |update VP(p1)
| 1            |    |    |      |     | p2  |   |   | p1|A(i)+=VP(i)
|              | p1 |    |      |     | p2  |   |   |   |A(p1)-= P

#### Validator Removal
Consider a new example with set.
Validator | p1 | p2 | p3 |
--------- |--- |--- |--- |
VP        | 1  | 2  | 3  |

Let's assume that the last round was R and the proposer priorities were as shown in first row (R end) with their sum being 0. At this point p2 is removed and round 1 of next height runs. At the end of the round (penultimate row) the sum of priorities is -2 (minus the priority of the removed process). 
 

|Priority Round |-3 | -2 | -1 | 0  | 1   | 2   | 3 | 4 | 5 | Comment
|---------------|-- | ---|--- |--- |---  |---  |---|---|---|--------
| R end         |p3 |    |    |    | p1  | p2  |   |   |   |remove p2
| 1             |   |    |    | p3 |     | p1  |   |   |   |A(i)+=VP(i)
|               |   | p1 |    | p3 |     |     |   |   |   |A(p1)-= P
| *new step*    |   |    | p1 |    | p3  |     |   |   |   |A(i) -= avg

The procedure could continue without modifications. However, it is possible that after a sufficiently large number of modifications in validator set, the priority values would group towards maximum or minimum allowed values.
For this reason, the selection procedure adds another step (see last row) that shifts the current priority values left or right such that the average remains 0.

The modified selection algorithm is:

    for each validator i in vset:
      A(i) += VP(i)
    prop = max(A)
    A(prop) -= P

    // normalize - shift with average priority
    avg = sum(A(i) for i in vset)/len(vset)
    for each validator i in vset:
      A(i) -= avg

#### New Validator
When a new validator is added same problem as the one described for removal appears, the sum of priorities in the new set is not zero. This is now fixed with the shift step introduced above.

One other issue that needs to be addressed is the following. A validator V with low voting power that has just been elected is moved to the end of the queue. If the validator set is large and/ or with other validators with significantly higher power, validator V will have to wait a lot of rounds to be elected. If V could remove and read itself to the validator set, it would make a significant (albeit unfair) "jump" ahead in the queue, decreasing the numbers of rounds it has to wait. 

In order to prevent this, when a new validator is added, its initial priority is not set to 0 but to a lower value. This value is determined from the simulation of the above scenario: it is assumed that V had just proposed a block and was at the back of the queue when it was removed and re-added. In addition, a penalty factor is applied.

If `vset = {v1,..,vn, V}` and `V` would be selected as proposer then it would have gone through the following changes:

    ...
      A(V)+=VP(V) // part of first for loop
    ...
    A(V)-=P  

It follows that `A(V)-=sum(VP(i) for i in {v1,..,vn})`

Therefore when `V` is added to the `{v1,..,vn}` set its initial priority will be:

    A(V) = -sum(VP(i) for i in {v1,..,vn})

With a penalty factor of 1.125, the initial priority for a newly added verifier is:
    
    A(V) = -1.125 * P

### Wrinkles

#### Validator Power Overflow Conditions
The validator voting power is a positive number stored as an int64. When a validator is added the `1.125 * P` computation must not overflow. As a consequence the code handling validator updates (add and update) checks for overflow conditions making sure the total voting power is never larger than the largest int64 `MAX` with the property that `1.125 * MAX` is still in the bounds of int64. Fatal error is return when overflow condition is detected.

#### Proposer Priority Overflow/ Underflow Handling
The proposer priority is stored as an int64. The selection algorithm performs additions and subtractions to these values and in the case of overflows and underflows it limits the values to:

    MaxInt64  = 1<<63 - 1
    MinInt64  = -1 << 63

-----------

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
