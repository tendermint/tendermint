# PBTS: System Model and Properties

## System Model

#### **[PBTS-CLOCK-NEWTON.0]**

There is a reference Newtonian real-time `t` (UTC).

No process has direct access to this reference time, used only for specification purposes.

### Synchronized clocks

Processes are assumed to be equipped with synchronized clocks.

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
for any two processes `p` and `q`, with local clocks `C_p` and `C_q`,
that read their local clocks at the same real-time `t`,  we have:

- If `p` and `q` are equipped with synchronized clocks, then `|C_p(t) - C_q(t)| < PRECISION`

`PRECISION` thus bounds the difference on the times simultaneously read by processes
from their local clocks, so that their clocks can be considered synchronized.

#### Accuracy

The [first draft][sysmodel_v1] of this specification included a second clock-related parameter, `ACCURACY`,
that relates the values read by processes from their synchronized clocks with real time:

- If `p` is a process is equipped with a synchronized clock, then at real time
  `t` it reads from its clock time `C_p(t)` with `|C_p(t) - t| < ACCURACY`

The adoption of `ACCURACY` as the upper bound on the difference between clock
readings and real time, however, renders the `PRECISION` parameter redundant.
In fact, if we assume that clocks readings are at most `ACCURACY` from real
time, we would therefore be assuming that they cannot be more than `2 * ACCURACY`
apart from each other, thus establishing a worst-case upper bound for `PRECISION`.

The approach we take is to assume that processes clocks are periodically
synchronized with an external source of time, thus improving their accuracy.
This allows us to adopt a relaxed version of the above `ACCURACY` definition:

##### **[PBTS-CLOCK-FAIR.0]**

- At real time `t` there is at least one correct process `p` which clock marks
  `C_p(t)` with `|C_p(t) - t| < ACCURACY`

Then, through [PBTS-CLOCK-PRECISION] we can extend this relation of clock times
with real time to every correct process, which will have a clock with accuracy
bound by `ACCURACY + PRECISION`.
But, for the sake of simpler specification we can assume that the `PRECISION`,
which is a worst-case parameter that applies to all correct processes,
includes the best `ACCURACY` achieved by any of them.

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

#### **[PBTS-MSG-D.0]**

There exists a system parameter `MSGDELAY` for end-to-end delays of messages carrying proposals,
such for any two correct processes `p` and `q`, and any real time `t`:

- If `p` sends a message `m` carrying a proposal at time `ts`,
then if `q` receives the message and learns the proposal,
`q` does that at time `t` such that `ts <= t <= ts + MSGDELAY`.

While we don't want to impose particular restrictions regarding the format of `m`,
we need to assume that their size is upper bounded.
In practice, using messages with a fixed-size to carry proposals allows
for a more accurate estimation of `MSGDELAY`, and therefore is advised.

## Problem Statement

In this section we define the properties of Tendermint consensus
(cf. the [arXiv paper][arXiv]) in this new system model.

#### **[PBTS-PROPOSE.0]**

A proposer proposes a consensus value `v` with an associated proposal time `v.time`.

#### **[PBTS-INV-AGREEMENT.0]**

[Agreement] No two correct processes decide on different values `v`. (This implies that no two correct processes decide on different proposal times `v.time`.)

#### **[PBTS-INV-VALID.0]**

[Validity] If a correct process decides on value `v`,
then `v` satisfies a predefined `valid` predicate.

#### **[PBTS-INV-TIMELY.0]**

[Time-Validity] If a correct process decides on value `v`,
then the associated proposal time `v.time` satisfies a predefined `timely` predicate.

> Both [Validity] and [Time-Validity] must be observed even if up to `2f` validators are faulty.

### Timely proposals

The `timely` predicate is evaluated when a process receives a proposal.
Let `now_p` be time a process `p` reads from its local clock when `p` receives a proposal.
Let `v` be the proposed value and `v.time` the proposal time.
The proposal is considered `timely` by `p` if:

#### **[PBTS-RECEPTION-STEP.1]**

1. `now_p >= v.time - PRECISION` and
1. `now_p <= v.time + MSGDELAY + PRECISION`

### Timely Proof-of-Locks

We denote by `POL(v,r)` a *Proof-of-Lock* of value `v` at the round `r` of consensus.
`POL(v,r)` consists of a set of `PREVOTE` messages of round `r` for the value `v`
from processes whose cumulative voting power is at least `2f + 1`.

#### **[PBTS-TIMELY-POL.1]**

If

- there is a valid `POL(v,r*)` for height `h`, and
- `r*` is the lowest-numbered round `r` of height `h` for which there is a valid `POL(v,r)`, and
- `POL(v,r*)` contains a `PREVOTE` message from at least one correct process,

Then, where `p` is a such correct process:

- `p` received a `PROPOSE` message of round `r*` and height `h`, and
- the `PROPOSE` message contained a proposal for value `v` with proposal time `v.time`, and
- a correct process `p` considered the proposal `timely`.

The round `r*` above defined will be, in most cases,
the round in which `v` was originally proposed, and when `v.time` was assigned,
using a `PROPOSE` message with `POLRound = -1`.
In any case, at least one correct process must consider the proposal `timely` at round `r*`
to enable a valid `POL(v,r*)` to be observed.

### Derived Proof-of-Locks

#### **[PBTS-DERIVED-POL.1]**

If

- there is a valid `POL(v,r)` for height `h`, and
- `POL(v,r)` contains a `PREVOTE` message from at least one correct process,

Then

- there is a valid `POL(v,r*)` for height `h`, with `r* <= r`, and
- `POL(v,r*)` contains a `PREVOTE` message from at least one correct process, and
- a correct process considered the proposal for `v` `timely` at round `r*`.

The above relation derives from a recursion on the round number `r`.
It is trivially observed when `r = r*`, the base of the recursion,
when a timely `POL(v,r*)` is obtained.
We need to ensure that, once a timely `POL(v,r*)` is obtained,
it is possible to obtain a valid `POL(v,r)` with `r > r*`,
without the need of satisfying the `timely` predicate (again) in round `r`.
In fact, since rounds are started in order, it is not likely that
a proposal time `v.time`, assigned at round `r*`,
will still be considered `timely` when the round `r > r*` is in progress.

In other words, the algorithm should ensure that once a `POL(v,r*)` attests
that the proposal for `v` is `timely`,
further valid `POL(v,r)` with `r > r*` can be obtained,
even though processes do not consider the proposal for `v` `timely` any longer.

> This can be achieved if the proposer of round `r' > r*` proposes `v` in a `PROPOSE` message
with `POLRound = r*`, and at least one correct processes is aware of a `POL(v,r*)`.
> From this point, if a valid `POL(v,r')` is achieved, it can replace the adopted `POL(v,r*)`.

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
