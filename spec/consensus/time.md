---
order: 2
---
# Block Time

Tendermint provides a deterministic, Byzantine fault-tolerant, source of time,
with bounded relation with the real time.

Time in Tendermint is defined by the `Time` field present in the header of
committed blocks, which satisfies the following properties:

- **Monotonicity**: `Time` is monotonically increasing. That is,
  given a header `H1` of a block at height `h1`
  and a header `H2` of a block at height `h2 > h1`,
  it is guaranteed that `H1.Time < H2.Time`.

- **Time-validity**: the `Time` of a block was considered valid, according with the
  [timely][pbts_timely] predicate, by at least one correct validator. This
  means that correct validators have attested that the block `Time` is compatible
  with the real time at which the block was produced.

The evaluation of the `timely` predicates involves comparing the `Time`
assigned to the block by its proposer with the local time at which a validator
receives the proposed block.
If the block Time and the block receive time are within the same *time window*,
the validator accepts the proposed block; otherwise, the block is rejected.
The *time window* boundaries are defined by two [consensus parameters][parameters]:

- `Precision`: the acceptable upper-bound of clock drift among all the
  validators on a Tendermint network. More precisely, the maximum acceptable
  difference on the time values simultaneous read by any two correct validators
  from their local clocks.

- `MessageDelay`: the maximum acceptable end-to-end delay for delivering a
  `Proposal` message to all correct validators.
  This is a fixed-size message broadcast by the proposer of block that
  includes the proposed block unique `BlockID` and `Time`.

Validators are therefore assumed to be equipped with **synchronized clocks**.
The `Precision` parameter defines the worst-case precision achieved by the
adopted clock synchronization mechanism.
For the sake of time validity, this parameter essentially restricts how much
"in the future" a block Time can be, while the `MessageDelay` parameter mainly
restricts how much "in the past" a block Time can be.

For additional information on how block Times are produced and validated,
refer to the Proposer-Based TimeStamps (PBTS) [spec][pbts].

[pbts]: ./proposer-based-timestamp/README.md
[pbts_timely]: ./proposer-based-timestamp/pbts-sysmodel_002_draft.md#pbts-timely0
[parameters]: ../abci/apps.md#consensus-parameters
