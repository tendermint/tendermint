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

#### R2: Fairness
In a stable network (no validator set changes) with total voting power P and for any sequence of K rounds with K > P, a validator v is elected as proposer with a frequency f proportional to its voting power VP(v) divided by P:

    f(v) ~ VP(v) / P

#### O3: Starvation Prevention
In a churning network with many validator set changes, minimal guarantees for a chance in future election should be offered, e.g. for stable lower power validators.

## Proposer Selection Specification
### Requirement Fulfillment Claims
__[R1]__ The proposer algorithm is deterministic giving consistent results across executions with same transactions and validator set modifications. A pseudocode of the algorithm is given below.  

__[R2]__ Given a set of processes with the total voting power P, during a sequence of rounds of length P, every process is selected as proposer in a number of rounds equal to its voting power. The sequence of the P proposers then repeats.
If we consider the validator set:

Validator | p1| p2 
----------|---|---
VP        | 1 | 3

The current implementation of proposer selection generates the sequence:
`p2, p1, p2, p2, p2, p1, p2, p2,...` or [`p2, p1, p2, p2`]*
A a sequence that starts with any circular permutation of the [`p2, p1, p2, p2`] sub-sequence would also provide the same degree of fairness. In fact these circular permutations show in the sliding window (over the generated sequence) of size equal to the length of the sub-sequence.

Assigning priorities to each validator based on the voting power and updating them at each round ensures the fairness of the proposer selection. In addition, every time a validator is elected as proposer its priority is decreased with the total voting power.

__[O3]__ There is currently no definite guarantee that a low power stable validator will be elected within a large enough round window (??)
Note: Just a claim here. If new high power validators bound and unbound continuously, and since there is no "aging" that would cause a steady increase in its voting power even when shifting is performed (or similar mechanism), it is possible that this validator election is postponed indefinitely. More analysis required either way.
[Maybe should remove this]

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
    def simpleProposerSelection (vset):
        for each validator i in vset:
            A(i) += VP(i)
        prop = max(A)
        A(prop) -= P
```

If the set of the validators has not changed, at the beginning of each new round the sum of their priorities is 0.

Following table shows, for the example above, how the proposer priority is calculated during each round. Only 4 rounds are shown, starting with the 5th the same values are computed.
Each row shows the imaginary queue and the process place in it. The proposer is the closest to the head of the queue, the rightmost validator. As priorities are updated, the validators move right in the queue. At the end of the round the proposer moves left as its priority is reduced after election. At the end of each round the sum of the priorities is 0.

|Round Priority  | -2| -1| 0   |  1| 2   | 3 | 4 | 5 | Alg step
|--------------- |---|---|---- |---|---- |---|---|---|--------
|                |   |   |p1,p2|   |     |   |   |   |Initialized to 0
|1               |   |   |     | p1|     | p2|   |   |A(i)+=VP(i)
|                |   | p2|     | p1|     |   |   |   |A(p2)-= P
|2               |   |   |     |   |p1,p2|   |   |   |A(i)+=VP(i)
|                | p1|   |     |   |   p2|   |   |   |A(p1)-= P
|3               |   | p1|     |   |     |   |   | p2|A(i)+=VP(i)
|                |   | p1|     | p2|     |   |   |   |A(p2)-= P
|4               |   |   |   p1|   |     |   | p2|   |A(i)+=VP(i)
|                |   |   |p1,p2|   |     |   |   |   |A(p2)-= P

The actual implementation does not create this queue, it only has to store the current accumulated proposer priority. 

There are cases where the proposer selection is executed multiple times.
(find a good example here)
So we modify slightly the algorithm to handle this case. Note that times indicates the number of executions of the simpleProposerSelection() 

    def singleProposerSelection (vset):
        for each validator i in vset:
            A(i) += VP(i)
        prop = max(A)
        A(prop) -= P
    
    def ProposerSelection (vset, times):
        for in in range(times):
            singleProposerSelection(vset)

### Validator Set Changes
At each block height the validator set may change. Some of the changes have implications on the proposer selection.

#### Voting Power Change
Consider again the earlier example and assume that the voting power of p1 is changed to 4:


Validator | p1| p2 
----------|---| ---
VP        | 4 | 3

Let's also assume that at the last round R at height H before this change the proposer priorites were as shown in first row (H/R end). As it can be seen, the procedure could continue with round 1 at next height without changes as before. 

|Round Priority| -2 | -1 | 0    | 1   | 2   | 3 | 4 | 5 | Comment
|--------------| ---|--- |------|---  |---  |---|---|---|--------
| H/R end      |    | p2 |      |  p1 |     |   |   |   |update VP(p1)
| H+1/0        |    |    |      |     | p2  |   |   | p1|A(i)+=VP(i)
|              | p1 |    |      |     | p2  |   |   |   |A(p1)-= P

However, when a validator changes power from a high to a low value it may be stuck at the end of the queue for a long time if it happened to be there (e.g. it just proposed). This scenario is considered again after the New Validator section.

#### Validator Removal
Consider a new example with set.

Validator | p1 | p2 | p3 |
--------- |--- |--- |--- |
VP        | 1  | 2  | 3  |

Let's assume that the last round at height H was R and the proposer priorities were as shown in first row (H/R end) with their sum being 0. At this point p2 is removed and round 1 of next height H+1 runs. At the end of the round (penultimate row) the sum of priorities is -2 (minus the priority of the removed process). 
 

|Round Priority |-3 | -2 | -1 | 0  | 1   | 2   | 3 | 4 | 5 | Comment
|---------------|-- | ---|--- |--- |---  |---  |---|---|---|--------
| H/R end       |p3 |    |    |    | p1  | p2  |   |   |   |remove p2
| H+1/0         |   |    |    | p3 |     | p1  |   |   |   |A(i)+=VP(i)
|               |   | p1 |    | p3 |     |     |   |   |   |A(p1)-= P
| *new step*    |   |    | p1 |    | p3  |     |   |   |   |A(i) -= avg

The procedure could continue without modifications. However, it is possible that after a sufficiently large number of modifications in validator set, the priority values would migrate towards maximum or minimum allowed values causing truncations due to overflow detection.
For this reason, the selection procedure adds another step (see last row) that shifts the current priority values left or right such that the average remains close to 0. Due to integer division the priority sum is in {-1, 0, 1}

The modified selection algorithm is:

    def singleProposerSelection (vset):
        for each validator i in vset:
            A(i) += VP(i)
        prop = max(A)
        A(prop) -= P
    
    def ProposerSelection (vset, times):
        for in in range(times):
            singleProposerSelection(vset)

        // shift priorities with the average
        avg = sum(A(i) for i in vset)/len(vset)
        for each validator i in vset:
            A(i) -= avg

Note that the shifting operation happens after singleProposerSelection() has run times times.

#### New Validator
When a new validator is added same problem as the one described for removal appears, the sum of priorities in the new set is not zero. This is now fixed with the shift step introduced above.

One other issue that needs to be addressed is the following. A validator V with low voting power that has just been elected is moved to the end of the queue. If the validator set is large and/ or with other validators with significantly higher power, validator V will have to wait a lot of rounds to be elected. If V could remove and re-add itself to the validator set, it would make a significant (albeit unfair) "jump" ahead in the queue, decreasing the numbers of rounds it has to wait. 

In order to prevent this, when a new validator is added, its initial priority is not set to 0 but to a lower value. This value is determined from the simulation of the above scenario: it is assumed that V had just proposed a block and was at the back of the queue when it was removed and re-added. In addition, to discourage intentional unbound/ bound operations, a penalty factor is applied.

If `vset = {v1,..,vn, V}` and `V` would be selected as proposer then it would have gone through the following changes:

    ...
      A(V)+=VP(V) // part of first for loop
    ...
    A(V)-=P  

It follows that `A(V)-=sum(VP(i) for i in {v1,..,vn})`

Therefore when `V` is added to the `{v1,..,vn}` set its initial priority will be:

    A(V) = -sum(VP(i) for i in {v1,..,vn})

Curent implementation uses a penalty factor of 1.125, so the initial priority for a newly added verifier is:
    
    A(V) = -1.125 * P

If we consider the validator set where p3 has just been added:

Validator | p1 | p2 | p3
----------|--- |--- |---
VP        | 1  | 3  | 8

Let's assume that the last round was R and the proposer priorities were as shown in first row (R end) with their sum being 0. p3 is added with A(p3) = -4 (penalty loss due to integer division?)
In the next round, p3 will still be ahead in the queue, elected as proposer and move back in the queue (same position it was just added)

|Round Priority  | -4 | -3 | -2 | -1 | 0  | 1 | 2 | 3 | 4 | Alg step
|--------------- | ---|--- |----|--- |--- |---|---|---|---|--------
|H/R end         |    |    | p2 |    |    |   | p1|   |   |add p3
|                | p3 |    | p2 |    |    |   | p1|   |   |A(p3) = -4
|H+1/0           |    |    |    |    |    | p2|   | p1| p3|A(i)+=VP(i)
|                | p3 |    |    |    |    | p2|   | p1|   |A(p3)-=P

### Proposer Priority Range
With the introduction of shifting, an interesting case occurs. Low power validators that bind early benefit from subsequent addition of high power validators. This is because new validators are added with negative priorities and this causes an increase in priority for all other validators during the shifting operation. As a consequence, low power validators may end up with high priority and election algorithm will fail with respect to fairness.

As an example, let's assume the following set, where p2 has just been added in the first row (H/R end)

Validator | p1| p2 
----------|---| ---
VP        | 10| 100

The proposer selection runs as described in the New Validator section. Notice the priority of p1 bumping from 10 to 15 because of the shifting. Then in the last row shown the power of p2 is changed to 1 at the end of H+1. 

|Round Priority| -21 | -16 | -15 |-11 | 0  | 10 | 15 | 25 | 89 |  Comment
|--------------| --- |---- |---- |--- |--- |--- |--- |--- |--- |---------
| H/R end      |     |     |     | p2 | p1 |    |    |    |    | added p2
| H+1/0        |     |     |     |    |    | p1 |    |    | p2 | A(i)+=VP(i)
|              | p2  |     |     |    |    | p1 |    |    |    | A(p2)-= P, P=110
|              |     | p2  |     |    |    |    | p1 |    |    | A(i) -= avg, avg=-5
| VP(p2)<-1    |     |     | p2  |    |    |    |    | p1 |    | A(i)+=VP(i)
...

In this example p2 happened to be at the end of the queue when its power changed to 1 and it will now crawl for ~40 rounds to catch up with p1 even though their power ratio is 1:10.

In order to prevent these types of cases, the selection algorithm ensures that the range of priority values is not bigger than two times the total voting power. 

The modified selection algorithm is:

    def singleProposerSelection (vset):
        for each validator i in vset:
            A(i) += VP(i)
        prop = max(A)
        A(prop) -= P
    
    def ProposerSelection (vset, times):
        // normalize the priority values
	    if max(A)-min(A) > 2 * P:
            for each validator i in vset:
		        A(i) = A(i)/2

        for in in range(times):
            singleProposerSelection(vset)

        // shift priorities with the average
        avg = sum(A(i) for i in vset)/len(vset)
        for each validator i in vset:
            A(i) -= avg

### Wrinkles

#### Validator Power Overflow Conditions
The validator voting power is a positive number stored as an int64. When a validator is added the `1.125 * P` computation must not overflow. As a consequence the code handling validator updates (add and update) checks for overflow conditions making sure the total voting power is never larger than the largest int64 `MAX`, with the property that `1.125 * MAX` is still in the bounds of int64. Fatal error is return when overflow condition is detected.

#### Proposer Priority Overflow/ Underflow Handling
The proposer priority is stored as an int64. The selection algorithm performs additions and subtractions to these values and in the case of overflows and underflows it limits the values to:

    MaxInt64  =  1 << 63 - 1
    MinInt64  = -1 << 63

-----------

## Formal Proofs
