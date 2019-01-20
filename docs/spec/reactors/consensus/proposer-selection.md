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
Given a validator set with total voting power P and a sequence S of elections. In any sub-sequence of S with length C*P, a validator v must be elected as proposer P/VP(v) times, i.e. with frequency:

 f(v) ~ VP(v) / P

where C is a tolerance factor for validator set changes with following values:
- C == 1 if there are no validator set changes
- C ~ k when there are k validator changes 

*[this needs more work]*

### Basic Algorithm

At its core, the proposer selection procedure uses a weighted round-robin algorithm.

A model that gives a good intuition on how/ why the selection algorithm works and it is fair is that of a priority queue. The validators move ahead in this queue according to their voting power (the higher the voting power the faster a validator moves towards the head of the queue). When the algorithm runs the following happens:
- all validators move "ahead" according to their powers: for each validator, increase the priority by the voting power
- first in the queue becomes the proposer: select the validator with highest priority 
- move the proposer back in the queue: decrease the proposer's priority by the total voting power

Notation:
- vset - the validator set
- A(i) - accumulated priority for validator i
- VP(i) - voting power of validator i
- P - total voting power of set
- avg - average of all validator priorities
- prop - proposer

Simple view at the Selection Algorithm:

```
    def ProposerSelection (vset):
        for each validator i in vset:
            A(i) += VP(i)
        prop = max(A)
        A(prop) -= P
```

### Stable Set

Consider the validator set:

Validator | p1| p2 
----------|---|---
VP        | 1 | 3

Assuming no validator changes, the following table shows the proposer priority computation over a few runs. Four runs of the selection procedure are shown, starting with the 5th the same values are computed.
Each row shows the priority queue and the process place in it. The proposer is the closest to the head, the rightmost validator. As priorities are updated, the validators move right in the queue. The proposer moves left as its priority is reduced after election. 

|Priority   Run  | -2| -1| 0   |  1| 2   | 3 | 4 | 5 | Alg step
|--------------- |---|---|---- |---|---- |---|---|---|--------
|                |   |   |p1,p2|   |     |   |   |   |Initialized to 0
|run 1           |   |   |     | p1|     | p2|   |   |A(i)+=VP(i)
|                |   | p2|     | p1|     |   |   |   |A(p2)-= P
|run 2           |   |   |     |   |p1,p2|   |   |   |A(i)+=VP(i)
|                | p1|   |     |   |   p2|   |   |   |A(p1)-= P
|run 3           |   | p1|     |   |     |   |   | p2|A(i)+=VP(i)
|                |   | p1|     | p2|     |   |   |   |A(p2)-= P
|run 4           |   |   |   p1|   |     |   | p2|   |A(i)+=VP(i)
|                |   |   |p1,p2|   |     |   |   |   |A(p2)-= P

It can pe shown that:
- At the end of each run k+1 the sum of the priorities is the same as at end of run k. If a new set's priorities are initialized to 0 then the sum of priorities will be 0 at each run while there are no changes.
- The max distance between priorites is P.

### Validator Set Changes
Between proposer selection runs the validator set may change. Some changes have implications on the proposer election.

#### Voting Power Change
Consider again the earlier example and assume that the voting power of p1 is changed to 4:

Validator | p1| p2 
----------|---| ---
VP        | 4 | 3

Let's also assume that before this change the proposer priorites were as shown in first row (last run). As it can be seen, the selection could run again, without changes, as before. 

|Priority   Run| -2 | -1 | 0    | 1   | 2   | Comment
|--------------| ---|--- |------|---  |---  |--------
| last run     |    | p2 |      |  p1 |     |__update VP(p1)__
| next run     |    |    |      |     | p2  |A(i)+=VP(i)
|              | p1 |    |      |     | p2  |A(p1)-= P

However, when a validator changes power from a high to a low value, some other validator remain far back in the queue for a long time. This scenario is considered again in the Proposer Priority Range section.

As before:
- At the end of each run k+1 the sum of the priorities is the same as at run k.
- The max distance between priorites is P.

#### Validator Removal
Consider a new example with set:

Validator | p1 | p2 | p3 |
--------- |--- |--- |--- |
VP        | 1  | 2  | 3  |

Let's assume that after the last run the proposer priorities were as shown in first row with their sum being 0. After p2 is removed, at the end of next proposer selection run (penultimate row) the sum of priorities is -2 (minus the priority of the removed process). 
 

|Priority   Run |-3 | -2 | -1 | 0  | 1   | 2   |Comment
|---------------|-- | ---|--- |--- |---  |---  |--------
| last run      |p3 |    |    |    | p1  | p2  |__remove p2__
| next run      |   |    |    | p3 |     | p1  |A(i)+=VP(i)
|               |   | p1 |    | p3 |     |     |A(p1)-= P
| __new step__  |   |    | p1 |    | p3  |     |A(i) -= avg, avg = -1

The procedure could continue without modifications. However, after a sufficiently large number of modifications in validator set, the priority values would migrate towards maximum or minimum allowed values causing truncations due to overflow detection.
For this reason, the selection procedure adds another __new step__ that shifts the current priority values left or right such that the average remains close to 0. 

The modified selection algorithm is:

    def ProposerSelection (vset):
        for each validator i in vset:
            A(i) += VP(i)
        prop = max(A)
        A(prop) -= P
    
        // shift priorities with the average
        avg = sum(A(i) for i in vset)/len(vset)
        for each validator i in vset:
            A(i) -= avg

Note that the shifting operation currently happens after ProposerSelection() has run.

Observations:
- The sum of priorities is now close to 0. Due to integer division the sum is an integer in (-n, n), where n is the number of validators.
- The max distance between priorites is now dependant on the history of the validator set changes (more in the Proposer Priority Range section).

#### New Validator
When a new validator is added same problem as the one described for removal appears, the sum of priorities in the new set is not zero. This is fixed with the shift step introduced above.

One other issue that needs to be addressed is the following. A validator V that has just been elected is moved to the end of the queue. If the validator set is large and/ or other validators have significantly higher power, V will have to wait a more runs to be elected. If V removes and re-adds itself to the set, it would make a significant (albeit unfair) "jump" ahead in the queue. 

In order to prevent this, when a new validator is added, its initial priority is not set to 0 but to a lower value. This value is determined from the simulation of the above scenario: it is assumed that V had just proposed a block and was at the back of the queue when it was removed and re-added. In addition, to discourage intentional unbound/ bound operations, a penalty factor is applied.

If `vset = {v1,..,vn, V}` and `V` would be selected as proposer then it would have gone through the following changes:

    ...
      A(V)+=VP(V) // part of first for loop
    ...
    A(V)-=P  

It follows that `A(V)-=sum(VP(i) for i in {v1,..,vn})`

Therefore when `V` is added to the `{v1,..,vn}` set its initial priority will be:

    A(V) = -sum(VP(i) for i in {v1,..,vn})

Curent implementation uses a penalty factor of 1.125, so the initial priority for a newly added validator is:
    
    A(V) ~ -1.125 * P

1.125 was chosen because it provides a small punishment that is efficient to calculate. See [here](https://github.com/tendermint/tendermint/pull/2785#discussion_r235038971) for more details.

If we consider the validator set where p3 has just been added:

Validator | p1 | p2 | p3
----------|--- |--- |---
VP        | 1  | 3  | 8

Assume that at the last run the proposer priorities were as shown in first row with their sum being 0. Then p3 is added with:

A(p3) = -1.125 * (1 + 3) ~ 4

Note that since current computation uses integer division there is penalty loss when sum of the voting power is less than 8.

In the next run, p3 will still be ahead in the queue, elected as proposer and moved back in the queue.

|Priority   Run | -8 | -7 | -4 | -3 | -2 | -1 | 0  | 1 | 2 | 3 | 4 | Alg step
|---------------|--- |--- |--- |--- |----|--- |--- |---|---|---|---|--------
|last run       |    |    |    |    | p2 |    |    |   | p1|   |   |__add p3__
|               |    |    | p3 |    | p2 |    |    |   | p1|   |   |A(p3) = -4
|next run       |    |    |    |    |    |    |    | p2|   | p1| p3|A(i)+=VP(i)
|               | p3 |    |    |    |    |    |    | p2|   | p1|   |A(p3)-=P
|               |    | p3 |    |    |    |    |    |   | p2|   | p1|A(i) -= avg, avg = -1

### Proposer Priority Range
With the introduction of shifting, some interesting cases occur. Low power validators that bind early in a set that includes high power validator(s) benefit from subsequent additions to the set. This is because these early validators run through more "right" shift operations that increase their priority.

As an example, consider the set where p2 is added after p1, with priority -1.125 * 80k = -90k. After the selection procedure runs once:

Validator | p1  | p2  | Comment
----------|-----|---- |---
VP        | 80k |  10 |
A         |  0  |-90k | __added p2__
A         |-45k | 45k | __run selection__

Then execute the following steps:

1. Add a new validator p3:

Validator | p1  | p2 | p3 
----------|-----|--- |----
VP        | 80k | 10 | 10 

2. Run selection once. The notation '..p' means very small increase in priority.

|Priority  Run | -90k..| -60k | -45k   | -15k| 0 | 45k  | 75k  | 125k   | Comment
|--------------|------ |----- |------- |-----|---|------ |------|------- |---------
| last run     |   p3  |      | p2     |     |   |   p1  |      |        | __added p3__
| nex run      |  ..p3 |      | ..p2   |     |   |       |      |   p1   | A(i)+=VP(i)
|              |  ..p3 |      | ..p2   |     |   | ..p1  |      |        | A(p1)-=P, P=80k+20
|*right_shift* |       |   p3 |        | p2  |   |       |  p1  |        | A(i) -= avg,avg=-30k


3. Remove p1 and run selection once:

Validator | p3   | p2  | Comment
----------|----- |---- |--------
VP        | 10   | 10  |
A         |-60k  |-15k |  
A         |-22.5k|22.5k| __run selection__

At this point, while the total voting power is 20, the distance between priorities is 45k. It will take 4500 runs for p3 to catch up with p2.

In order to prevent these types of scenarios, the selection algorithm performs normalization of priorities such that the difference between min and max values is smaller than two times the total voting power. 

The modified selection algorithm is:

    def ProposerSelection (vset):
        // normalize the priority values
        diff = max(A)-min(A)
        threshold = 2 * P
	    if  diff > threshold:
            scale = diff/threshold
            for each validator i in vset:
		        A(i) = A(i)/scale

        for each validator i in vset:
            A(i) += VP(i)
        prop = max(A)
        A(prop) -= P

        // shift priorities with the average
        avg = sum(A(i) for i in vset)/len(vset)
        for each validator i in vset:
            A(i) -= avg

Observations:
- With this modification, the max distance between priorites becomes 2 * P.

### Wrinkles

#### Validator Power Overflow Conditions
The validator voting power is a positive number stored as an int64. When a validator is added the `1.125 * P` computation must not overflow. As a consequence the code handling validator updates (add and update) checks for overflow conditions making sure the total voting power is never larger than the largest int64 `MAX`, with the property that `1.125 * MAX` is still in the bounds of int64. Fatal error is return when overflow condition is detected.

#### Proposer Priority Overflow/ Underflow Handling
The proposer priority is stored as an int64. The selection algorithm performs additions and subtractions to these values and in the case of overflows and underflows it limits the values to:

    MaxInt64  =  1 << 63 - 1
    MinInt64  = -1 << 63

### Requirement Fulfillment Claims
__[R1]__ 

The proposer algorithm is deterministic giving consistent results across executions with same transactions and validator set modifications. 
[WIP - needs more detail]

__[R2]__ 
#### Constant Validator Set
Given a set of processes with the total voting power P, during a sequence of elections of length P, the number of times any process is selected as proposer is equal to its voting power. The sequence of the P proposers then repeats. If we consider the validator set:

Validator | p1| p2 
----------|---|---
VP        | 1 | 3

The current implementation of proposer selection generates the sequence:
`p2, p1, p2, p2, p2, p1, p2, p2,...` or [`p2, p1, p2, p2`]*
A sequence that starts with any circular permutation of the [`p2, p1, p2, p2`] sub-sequence would also provide the same degree of fairness. In fact these circular permutations show in the sliding window (over the generated sequence) of size equal to the length of the sub-sequence.

Assigning priorities to each validator based on the voting power and updating them at each run ensures the fairness of the proposer selection. In addition, every time a validator is elected as proposer its priority is decreased with the total voting power.

Intuitively, a process p jumps ahead in the queue ~P/VP(i) times until it reaches the head and is elected. 

#### Changed Validator Set
Current implementation normalizes the priorities after every change such that the distance between min and max is at most 2 * P. If no other change occurs, a process p needs to move ahead in the queue ~ 2 * P/VP(i) times to reach the head and be elected. 

-----------

## Formal Proofs
Notations:
- n - the number of validators in vset
- vset - the validator set {p<sub>i</sub>, i=1..n}
- v<sub>i</sub> - the voting power of p<sub>i</sub>
- a<sub>i,k</sub>  - the priority of p<sub>i</sub> in round k
- A<sub>k</sub> = {a<sub>i,k</sub>, i=1..n} - the priority values in round k
- dist(A<sub>k</sub>) - difference between min and max over values in A<sub>k</sub>

### Stable Validator Set
Assuming that there are no validator set changes between rounds k and k+1.

__T1__:  sum(a<sub>i,k+1</sub>) = sum(a<sub>i,k</sub>) 

__P1__: In round k+1:
- priorities are increased with v<sub>i</sub> for all p<sub>i</sub>:
    - a<sub>i,k+1</sub> = a<sub>i,k</sub> + v<sub>i</sub>
    - => sum(a<sub>i,k+1</sub>) = sum(a<sub>i,k</sub>) + sum(v<sub>i</sub>)
- priority of proposer p is decreased with total voting power:
    - a<sub>p,k+1</sub> = a<sub>p,k+1</sub> - sum(v<sub>i</sub>)
    - => sum(a<sub>i,k+1</sub>) = sum(a<sub>i,k+1</sub>) - sum(v<sub>i</sub>) = sum(a<sub>i,k</sub>)

In a new set a<sub>i,0</sub> = 0. Therefore sum(a<sub>i,k</sub>) = 0 while there are no changes.

__T2__: dist(A<sub>k+1</sub>) = max(dist(A<sub>k</sub>), sum(v<sub>i</sub>))

__P2__: [WIP]


### Update Voting Power

Let v<sub>i, k</sub> be the voting power of p<sub>i</sub> in round k.

__T3__:  When a validator p<sub>q</sub> changes its voting power just before k+1 to v<sub>q,k+1</sub> = v<sub>q,k</sub> + X then __T1__ still holds:

-  sum(a<sub>i,k+1</sub>) = sum(a<sub>i,k</sub>) 

__P3__: 

In round k+1 when priorities are increased:
- a<sub>i,k+1</sub> = a<sub>i,k</sub> + v<sub>i,k</sub>, for i <> q
- a<sub>q,k+1</sub> = a<sub>q,k</sub> + v<sub>i,k</sub> + X

=> sum(a<sub>i,k+1</sub>) = sum(a<sub>i,k</sub>) + sum(v<sub>i,k</sub>) + X

Then the priority of proposer p is decreased with new total voting power:
- a<sub>p,k+1</sub> = a<sub>p,k+1</sub> - (sum(v<sub>i, k</sub>) + X)

 => sum(a<sub>i,k+1</sub>) = sum(a<sub>i,k+1</sub>) - sum(v<sub>i, k</sub>) - X = sum(a<sub>i,k</sub>)
