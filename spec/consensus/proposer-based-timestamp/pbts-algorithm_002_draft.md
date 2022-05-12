# PBTS: Protocol Specification

## Proposal Time

PBTS computes for a proposed value `v` the proposal time `v.time`, with bounded difference to the actual real-time the proposed value was generated.
The proposal time is read from the clock of the process that proposes a value for the first time, its original proposer.

With PBTS, therefore, we assume that processes have access to **synchronized clocks**.
The proper definition of what it means can be found in the [system model][sysmodel],
but essentially we assume that two correct processes do not simultaneous read from their clocks
time values that differ more than `PRECISION`, which is a system parameter.

### Proposal times are definitive

When a value `v` is produced by a process, it also assigns the associated proposal time `v.time`.
If the same value `v` is then re-proposed in a subsequent round of consensus,
it retains its original time, assigned by its original proposer.

A value `v` is re-proposed when it becomes a valid value, i.e., when it receives `2f + 1 PREVOTE`s in a round `r` of consensus.
This means that processes with `2f + 1`-equivalent voting power accepted, in round `r`, both `v` and its associated time `v.time`.
Since the originally proposed value and its associated time were considered valid, there is no reason for reassigning `v.time`.

## Time Monotonicity

Values decided in successive heights of consensus must have increasing times, so:

- Monotonicity: for any process `p` and any two decided heights `h` and `h'`, if `h > h'` then <code>decision<sub>p</sub>[h].time > decision<sub>p</sub>[h'].time</code>.

For ensuring time monotonicity, it is enough to ensure that a value `v` proposed by process `p` at height <code>h<sub>p</sub></code> has <code>v.time > decision<sub>p</sub>[h<sub>p</sub>-1].time</code>.
So, if process `p` is the proposer of a round of height <code>h<sub>p</sub></code> and reads from its clock a time <code>now<sub>p</sub> <= decision<sub>p</sub>[h<sub>p</sub>-1]</code>,
it should postpone the generation of its proposal until <code>now<sub>p</sub> > decision<sub>p</sub>[h<sub>p</sub>-1]</code>.

> Although it should be considered, this scenario is unlikely during regular operation,
as from <code>decision<sub>p</sub>[h<sub>p</sub>-1].time</code> and the start of height <code>h<sub>p</sub></code>, a complete consensus instance need to terminate.

The time monotonicity of values proposed in heights of consensus is verified by the `valid()` predicate, to which every proposed value is submitted.
A value rejected by the `valid()` implementation is not accepted by any correct process.

## Timely Proposals

PBTS introduces a new requirement for a process to accept a proposal: the proposal must be `timely`.
It is a temporal requirement, associated with the following synchrony (that is, timing)
[assumptions][sysmodel] regarding the behavior of processes and the network:

- Synchronized clocks: the values simultaneously read from clocks of any two correct processes differ by at most `PRECISION`;
- Bounded transmission delays: the real time interval between the sending of a proposal at a correct process, and the reception of the proposal at any correct process is upper bounded by `MSGDELAY`.

#### **[PBTS-RECEPTION-STEP.1]**

Let <code>now<sub>p</sub></code> be the time, read from the clock of process `p`, at which `p` receives the proposed value `v`.
The proposal is considered `timely` by `p` when:

1. <code>now<sub>p</sub> >= v.time - PRECISION</code>
1. <code>now<sub>p</sub> <= v.time + MSGDELAY + PRECISION</code>

The first condition derives from the fact that the generation and sending of `v` precedes its reception.
The minimum receiving time <code>now<sub>p</sub></code> for `v` be considered `timely` by `p` is derived from the extreme scenario when
the clock of `p` is `PRECISION` *behind* of the clock of the proposer of `v`, and the proposal's transmission delay is `0` (minimum).

The second condition derives from the assumption of an upper bound for the transmission delay of a proposal.
The maximum receiving time <code>now<sub>p</sub></code> for `v` be considered `timely` by `p` is derived from the extreme scenario when
the clock of `p` is `PRECISION` *ahead* of the clock of the proposer of `v`, and the proposal's transmission delay is `MSGDELAY` (maximum).

## Updated Consensus Algorithm

The following changes are proposed for the algorithm in the [arXiv paper][arXiv].

#### New `StartRound` (Lines 11 - 21)

There are two additions to the `propose` round step when executed by the `proposer` of a round:

1. The proposer does not propose a value until its current local time becomes greater than the previously decided value's time;
1. When the proposer produces a new proposal, it sets the proposal's time to its current local time;
   - No changes are made to the logic when a proposer re-proposes its `validValue`, which retains its original proposal time.

```go
function StartRound(round) {
 round_p ← round
 step_p ← propose
 if proposer(h_p, round_p) = p {
  wait until now_p > decision_p[h_p-1].time // time monotonicity
  if validValue_p != nil {
   proposal ← validValue_p
  } else {
   proposal ← getValue()
   proposal.time ← now_p // proposal time
  }
   broadcast ⟨PROPOSAL, h_p, round_p, proposal, validRound_p⟩
 } else {
  schedule OnTimeoutPropose(h_p,round_p) to be executed after timeoutPropose(round_p)
 }
}
```

#### New rule for accepting values (Lines 22 - 27)

The rule on line 22 applies to values `v` proposed for the first time, i.e., for proposals not backed by `PREVOTE`s for `v` in a previous round `vr < r`.
The `PROPOSAL` message, in this case, carry `-1` in its `validRound` field.

The new rule for issuing a `PREVOTE` for a proposed value `v` requires the value to be `timely`.
The `timely` predicate is evaluated in the moment that the value is received,
as part of a `PROPOSAL` message, by comparing the process local time with `v.time`.

```go
upon ⟨PROPOSAL, h_p, round_p, v, −1⟩ from proposer(h_p, round_p) while step_p = propose do {
  if timely(v.time) ∧ valid(v) ∧ (lockedRound_p = −1 ∨ lockedValue_p = v) {
    broadcast ⟨PREVOTE, h_p, round_p, id(v)⟩ 
  }
  else {
    broadcast ⟨PREVOTE, h_p, round_p, nil⟩ 
  }
  step_p ← prevote
}
```

#### All other rules remains unchanged

Notice, in particular, that the rule on line 28 for values re-proposed, and backed by `2f + 1`-equivalent voting power `PREVOTE` messages, is not affected by the changes.

Back to [main document][main].

[main]: ./README.md

[sysmodel]: ./pbts-sysmodel_002_draft.md

[bfttime]: https://github.com/tendermint/tendermint/blob/master/spec/consensus/bft-time.md
[arXiv]: https://arxiv.org/pdf/1807.04938.pdf
