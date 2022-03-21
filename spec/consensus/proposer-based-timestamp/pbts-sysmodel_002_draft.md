# PBTS: System Model and Properties

## Outline

 - [System model](#system-model)
   - [Synchronized clocks](#synchronized-clocks)
   - [Message delays](#message-delays)
 - [Problem Statement](#problem-statement)
 - [Timely Proposals](#timely-proposals)
    - [Timely Proof-of-Locks](#timely-proof-of-locks)
    - [Derived Proof-of-Locks](#derived-proof-of-locks)
 - [Safety](#safety)
 - [Liveness](#liveness)

## System Model

#### **[PBTS-CLOCK-NEWTON.0]**

There is a reference Newtonian real-time `t`.

No process has direct access to this reference time, used only for specification purposes.
The reference real-time is assumed to be aligned with the Coordinated Universal Time (UTC).

### Synchronized clocks

Processes are assumed to be equipped with synchronized clocks,
aligned with the Coordinated Universal Time (UTC).

This requires processes to periodically synchronize their local clocks with an
external and trusted source of the time (e.g. NTP servers).
Each synchronization cycle aligns the process local clock with the external
source of time, making it a *fairly accurate* source of real time.
The periodic (re)synchronization aims to correct the *drift* of local clocks,
which tend to pace slightly faster or slower than the real time.

To avoid an excessive level detail in the parameters and guarantees of
synchronized clocks, we adopt a single system parameter `PRECISION` to
encapsulate the potential inaccuracy of the synchronization mechanisms,
and drifts of local clocks from real time.

#### **[PBTS-CLOCK-PRECISION.0]**

There exists a system parameter `PRECISION`, such that
for any two processes `p` and `q`, with local clocks `C_p` and `C_q`:

- If `p` and `q` are equipped with synchronized clocks,
 then for any real-time `t` we have `|C_p(t) - C_q(t)| <= PRECISION`.

`PRECISION` thus bounds the difference on the times simultaneously read by processes
from their local clocks, so that their clocks can be considered synchronized.

#### Accuracy

A second relevant clock parameter is accuracy, which binds the values read by
processes from their clocks to real time.

##### **[PBTS-CLOCK-ACCURACY.0]**

For the sake of completeness, we define a parameter `ACCURACY` such that:

- At real time `t` there is at least one correct process `p` which clock marks
  `C_p(t)` with `|C_p(t) - t| <= ACCURACY`.

As a consequence, applying the definition of `PRECISION`, we have:

- At real time `t` the synchronized clock of any correct process `p` marks
  `C_p(t)` with `|C_p(t) - t| <= ACCURACY + PRECISION`.

The reason for not adopting `ACCURACY` as a system parameter is the assumption
that `PRECISION >> ACCURACY`.
This allows us to consider, for practical purposes, that the `PRECISION` system
parameter embodies the `ACCURACY` model parameter.

### Message Delays

The assumption that processes have access to synchronized clocks ensures that proposal times
assigned by *correct processes* have a bounded relation with the real time.
It is not enough, however, to identify (and reject) proposal times proposed by Byzantine processes.

To properly evaluate whether the time assigned to a proposal is consistent with the real time,
we need some information regarding the time it takes for a message carrying a proposal
to reach all its (correct) destinations.
More precisely, the *maximum delay* for delivering a proposal to its destinations allows
defining a lower bound, a *minimum time* that a correct process assigns to proposal.
While *minimum delay* for delivering a proposal to a destination allows defining
an upper bound, the *maximum time* assigned to a proposal.

#### **[PBTS-MSG-DELAY.0]**

There exists a system parameter `MSGDELAY` for end-to-end delays of proposal messages,
such for any two correct processes `p` and `q`:

- If `p` sends a proposal message `m` at real time `t` and `q` receives `m` at
  real time `t'`, then `t <= t' <= t' + MSGDELAY`.

Notice that, as a system parameter, `MSGDELAY` should be observed for any
proposal message broadcast by correct processes: it is a *worst-case* parameter.
As message delays depends on the message size, the above requirement implicitly
indicates that the size of proposal messages is either fixed or upper bounded.

## Problem Statement

In this section we define the properties of Tendermint consensus
(cf. the [arXiv paper][arXiv]) in this system model.

### **[PBTS-PROPOSE.0]**

A proposer proposes a consensus value `v` that includes a proposal time
`v.time`.

> We then restrict the allowed decisions along the following lines:

#### **[PBTS-INV-AGREEMENT.0]**

- [Agreement] No two correct processes decide on different values `v`.

This implies that no two correct processes decide on different proposal times
`v.time`.

#### **[PBTS-INV-VALID.0]**

- [Validity] If a correct process decides on value `v`, then `v` satisfies a
  predefined `valid` predicate.

With respect to PBTS, the `valid` predicate requires proposal times to be
[monotonic](./pbts-algorithm_002_draft.md#time-monotonicity) over heights of
consensus:

##### **[PBTS-INV-MONOTONICITY.0]**

- If a correct process decides on value `v` at the height `h` of consensus,
  thus setting `decision[h] = v`, then `v.time > decision[h'].time` for all
  previous heights `h' < h`.

The monotonicity of proposal times, and external validity in general,
implicitly assumes that heights of consensus are executed in order.

#### **[PBTS-INV-TIMELY.0]**

- [Time-Validity] If a correct process decides on value `v`, then the proposal
  time `v.time` was considered `timely` by at least one correct process.

PBTS introduces a `timely` predicate that restricts the allowed decisions based
on the proposal time `v.time` associated with a proposed value `v`.
As a synchronous predicate, the time at which it is evaluated impacts on
whether a process accepts or reject a proposal time.
For this reason, the Time-Validity property refers to the previous evaluation
of the `timely` predicate, detailed in the following section.

## Timely proposals

For PBTS, a `proposal` is a tuple `(v, v.time, v.round)`, where:

- `v` is the proposed value;
- `v.time` is the associated proposal time;
- `v.round` is the round at which `v` was first proposed.

We include the proposal round `v.round` in the proposal definition because a
value `v` and its associated proposal time `v.time` can be proposed in multiple
rounds, but the evaluation of the `timely` predicate is only relevant at round
`v.round`.

> Considering the algorithm in the [arXiv paper][arXiv], a new proposal is
> produced by the `getValue()` method, invoked by the proposer `p` of round
> `round_p` when starting its proposing round with a nil `validValue_p`.
> The first round at which a value `v` is proposed is then the round at which
> the proposal for `v` was produced, and broadcast in a `PROPOSAL` message with
> `vr = -1`.

#### **[PBTS-PROPOSAL-RECEPTION.0]**

The `timely` predicate is evaluated when a process receives a proposal.
More precisely, let `p` be a correct process:

- `receiveTime_p[r]` is the time `p` reads from its local clock when `p` is at
  round `r` and receives the proposal of round `r`.

#### **[PBTS-TIMELY.0]**

The proposal `(v, v.time, v.round)` is considered `timely` by a correct process
`p` if:

1. `receiveTime_p[v.round]` is set, and
1. `receiveTime_p[v.round] >= v.time - PRECISION`, and
1. `receiveTime_p[v.round] <= v.time + MSGDELAY + PRECISION`.

A correct process at round `v.round` only sends a `PREVOTE` for `v` if the
associated proposal time `v.time` is considered `timely`.

> Considering the algorithm in the [arXiv paper][arXiv], the `timely` predicate
> is evaluated by a process `p` when it receives a valid `PROPOSAL` message
> from the proposer of the current round `round_p` with `vr = -1`.

### Timely Proof-of-Locks

A *Proof-of-Lock* is a set of `PREVOTE` messages of round of consensus for the
same value from processes whose cumulative voting power is at least `2f + 1`.
We denote as `POL(v,r)` a proof-of-lock of value `v` at round `r`.

For PBTS, we are particularly interested in the `POL(v,v.round)` produced in
the round `v.round` at which a value `v` was first proposed.
We call it a *timely* proof-of-lock for `v` because it can only be observed
if at least one correct process considered it `timely`:

#### **[PBTS-TIMELY-POL.0]**

If

- there is a valid `POL(v,r)` with `r = v.round`, and
- `POL(v,v.round)` contains a `PREVOTE` message from at least one correct process,

Then, let `p` is a such correct process:

- `p` received a `PROPOSAL` message of round `v.round`, and
- the `PROPOSAL` message contained a proposal `(v, v.time, v.round)`, and
- `p` was in round `v.round` and evaluated the proposal time `v.time` as `timely`.

The existence of a such correct process `p` is guaranteed provided that the
voting power of Byzantine processes is bounded by `2f`.

### Derived Proof-of-Locks

The existence of `POL(v,r)` is a requirement for the decision of `v` at round
`r` of consensus.

At the same time, the Time-Validity property establishes that if `v` is decided
then a timely proof-of-lock `POL(v,v.round)` must have been produced.

So, we need to demonstrate here that any valid `POL(v,r)` is either a timely
proof-of-lock or it is derived from a timely proof-of-lock:

#### **[PBTS-DERIVED-POL.0]**

If

- there is a valid `POL(v,r)`, and
- `POL(v,r)` contains a `PREVOTE` message from at least one correct process,

Then

- there is a valid `POL(v,v.round)` with `v.round <= r` which is a timely proof-of-lock.

The above relation is trivially observed when `r = v.round`, as `POL(v,r)` must
be a timely proof-of-lock.
Notice that we cannot have `r < v.round`, as `v.round` is defined as the first
round at which `v` was proposed.

For `r > v.round` we need to demonstrate that if there is a valid `POL(v,r)`,
then a timely `POL(v,v.round)` was previously obtained.
We observe that a condition for observing a `POL(v,r)` is that the proposer of
round `r` has broadcast a `PROPOSAL` message for `v`.
As `r > v.round`, we can affirm that `v` was not produced in round `r`.
Instead, by the protocol operation, `v` was a *valid value* for the proposer of
round `r`, which means that if the proposer has observed a `POL(v,vr)` with `vr
< r`.
The above operation considers a *correct* proposer, but since a `POL(v,r)` was
produced (by hypothesis) we can affirm that at least one correct process (also)
observed a `POL(v,vr)`.

> Considering the algorithm in the [arXiv paper][arXiv], `v` was proposed by
> the proposer `p` of round `round_p` because its `validValue_p` variable was
> set to `v`.
> The `PROPOSAL` message broadcast by the proposer, in this case, had `vr > -1`,
> and it could only be accepted by processes that also observed a `POL(v,vr)`.

Thus, if there is a `POL(v,r)` with `r > v.round`, then there is a valid
`POL(v,vr)` with `v.round <= vr < r`.
If `vr = v.round` then `POL(vr,v)` is a timely proof-of-lock and we are done.
Otherwise, there is another valid `POL(v,vr')` with `v.round <= vr' < vr`,
and the same reasoning can be applied until we get `vr' = v.round` and observe
a timely proof-of-lock (which is guaranteed, as `vr` necessarily decreases at
each recursive iteration).

### SAFETY

The safety of the algorithm requires a *timely* proof-of-lock for a decided value,
either directly evaluated by a correct process,
or indirectly received through a derived proof-of-lock.

#### **[PBTS-CONSENSUS-TIME-VALID.0]**

If

- there is a valid commit `C` for height `k` and round `r`, and
- `C` contains a `PRECOMMIT` message from at least one correct process

Then, where `p` is one such correct process:

- since `p` is correct, `p` received a valid `POL(v,r)`, and
- `POL(v,r)` contains a `PREVOTE` message from at least one correct process, and
- `POL(v,r)` is derived from a timely `POL(v,r*)` with `r* <= r`, and
- `POL(v,r*)` contains a `PREVOTE` message from at least one correct process, and
- a correct process considered a proposal for `v` `timely` at round `r*`.

### LIVENESS

In terms of liveness, we need to ensure that a proposal broadcast by a correct process
will be considered `timely` by any correct process that is ready to accept that proposal.
So, if:

- the proposer `p` of a round `r` is correct,
- there is no `POL(v',r')` for any value `v'` and any round `r' < r`,
- `p` proposes a valid value `v` and sets `v.time` to the time it reads from its local clock,

Then let `q` be a correct process that receives `p`'s proposal, we have:

- `q` receives `p`'s proposal after its clock reads `v.time - PRECISION`, and
- if `q` is at or joins round `r` while `p`'s proposal is being transmitted,
then `q` receives `p`'s proposal before its clock reads `v.time + MSGDELAY + PRECISION`

> Note that, before `GST`, we cannot ensure that every correct process receives `p`'s proposals, nor that it does it while ready to accept a round `r` proposal.

A correct process `q` as above defined must then consider `p`'s proposal `timely`.
It will then broadcast a `PREVOTE` message for `v` at round `r`,
thus enabling, from the Time-Validity point of view, `v` to be eventually decided.

#### Under-estimated `MSGDELAY`s

The liveness assumptions of PBTS are conditioned by a conservative and clever
choice of the timing parameters, specially of `MSGDELAY`.
In fact, if the transmission delay for a message carrying a proposal is wrongly
estimated, correct processes may never consider a valid proposal as `timely`.

To circumvent this liveness issue, which could result from a misconfiguration,
we assume that the `MSGDELAY` parameter can be increased as rounds do not
succeed on deciding a value, possibly because no proposal is considered
`timely` by enough processes.
The precise behavior for this workaround is under [discussion](https://github.com/tendermint/spec/issues/371).

Back to [main document][main].

[main]: ./README.md

[algorithm]: ./pbts-algorithm_002_draft.md

[sysmodel]: ./pbts-sysmodel_002_draft.md
[sysmodel_v1]: ./v1/pbts-sysmodel_001_draft.md

[arXiv]: https://arxiv.org/pdf/1807.04938.pdf
