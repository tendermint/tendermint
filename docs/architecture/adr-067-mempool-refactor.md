# ADR 067: Mempool Refactor

- [ADR 067: Mempool Refactor](#adr-067-mempool-refactor)
  - [Changelog](#changelog)
  - [Status](#status)
  - [Context](#context)
  - [Alternative Approaches](#alternative-approaches)
  - [Decision](#decision)
  - [Detailed Design](#detailed-design)
    - [CheckTx](#checktx)
    - [Mempool](#mempool)
    - [Eviction](#eviction)
    - [Gossiping](#gossiping)
  - [Consequences](#consequences)
    - [Positive](#positive)
    - [Negative](#negative)
    - [Neutral](#neutral)
  - [References](#references)

## Changelog

- April 19, 2021: Initial Draft (@alexanderbez)

## Status

Proposed

## Context

Tendermint Core has a reactor and data structure, mempool, that facilitates the
ephemeral storage of uncommitted transactions. Honest nodes participating in a
Tendermint network gossip these uncommitted transactions to each other if they
pass the application's `CheckTx`. In addition, block proposers select from the
mempool a subset of uncommitted transactions to include in the next block.

Currently, the mempool in Tendermint Core is designed as a FIFO queue. In other
words, transactions are included in blocks as they are received by a node. There
currently is no explicit and prioritized ordering of these uncommitted transactions.
This presents a few technical and UX challenges for operators and applications.

Namely, validators are not able to prioritize transactions by their fees or any
incentive aligned mechanism. In addition, the lack of prioritization also leads
to cascading effects in terms of DoS and various attack vectors on networks,
e.g. [cosmos/cosmos-sdk#8224](https://github.com/cosmos/cosmos-sdk/discussions/8224).

Thus, Tendermint Core needs the ability for an application and its users to
prioritize transactions in a flexible and performant manner. Specifically, we're
aiming to either improve, maintain or add the following properties in the
Tendermint mempool:

- Allow application-determined transaction priority.
- Allow efficient concurrent reads and writes.
- Allow block proposers to reap transactions efficiently by priority.
- Maintain a fixed mempool capacity by transaction size and evict lower priority
  transactions to make room for higher priority transactions.
- Allow transactions to be gossiped by priority efficiently.
- Allow operators to specify a maximum TTL for transactions in the mempool before
  they're automatically evicted if not selected for a block proposal in time.
- Ensure the design allows for future extensions, such as replace-by-priority and
  allowing multiple pending transactions per sender, to be incorporated easily.

Note, not all of these properties will be addressed by the proposed changes in
this ADR. However, this proposal will ensure that any unaddressed properties
can be addressed in an easy and extensible manner in the future.

## Alternative Approaches

When considering which approach to take for a priority-based flexible and
performant mempool, there are two core candidates. The first candidate in less
invasive in the required  set of protocol and implementation changes, which
simply extends the existing `CheckTx` ABCI method. The second candidate essentially
involves the introduction of new ABCI method(s) and would require a higher degree
of complexity in protocol and implementation changes, some of which may either
overlap or conflict with the upcoming introduction of [ABCI++](https://github.com/tendermint/spec/blob/master/rfc/004-abci%2B%2B.md).

For more information on the various approaches and proposals, please see the
[mempool discussion](https://github.com/tendermint/tendermint/discussions/6295).

## Decision

To incorporate a priority-based flexible and performant mempool in Tendermint Core,
we will introduce new fields, `priority` and `sender`, into the `ResponseCheckTx`
type and augment the existing mempool data structure to facilitate prioritization
of uncommitted transactions in addition to extended functionality such as
replace-by-priority and allowing multiple transactions to exist from the same
sender with varying priorities.

## Detailed Design

### CheckTx

We introduce the following new fields into the `ResponseCheckTx` type:

```diff
message ResponseCheckTx {
  uint32         code       = 1;
  bytes          data       = 2;
  string         log        = 3;  // nondeterministic
  string         info       = 4;  // nondeterministic
  int64          gas_wanted = 5 [json_name = "gas_wanted"];
  int64          gas_used   = 6 [json_name = "gas_used"];
  repeated Event events     = 7 [(gogoproto.nullable) = false, (gogoproto.jsontag) = "events,omitempty"];
  string         codespace  = 8;
+ int64          priority   = 9;
+ string         sender     = 10;
}
```

It is entirely up the application in determining how these fields are populated
and with what values, e.g. the `sender` could be the signer and fee payer 
the transaction, the `priority` could be the cumulative sum of the fee(s).

Only `sender` is required, while `priority` can be omitted which would result in
using the default value of zero.

### Mempool

The existing concurrent-safe linked-list will be removed entirely in favor of a
thread-safe map of `<sender:*Tx>`, i.e a mapping from `sender` to a single `*Tx`
object, where each `*Tx` is the next valid and processable transaction from the
given `sender`.

On top of this mapping, we index all transactions by priority using a thread-safe
priority queue, i.e. a [max heap](https://en.wikipedia.org/wiki/Min-max_heap).
When a proposer is ready to select transactions for the next block proposal,
transactions are selected from this priority index by highest priority order.
When a transaction is selected and reaped, it is removed from this index and
from the `<sender:*Tx>` mapping.

We define `Tx` as the following data structure:

```go
type Tx struct {
  // Tx represents the raw binary transaction data.
  Tx []byte

  // Priority defines the transaction's priority as specified by the application
  // in the ResponseCheckTx response.
  Priority int64

  // Sender defines the transaction's sender as specified by the application in
  // the ResponseCheckTx response.
  Sender string

  // Index defines the current index in the priority queue index. Note, if
  // multiple Tx indexes are needed, this field will be removed and each Tx
  // index will have its own wrapped Tx type.
  Index int
}
```

### Eviction

Upon successfully executing `CheckTx` for a new `Tx` and the mempool is currently
full, we must check if there exists a `Tx` of lower priority that can be evicted
to make room for the new `Tx` with higher priority and with sufficient size
capacity left.

If such a `Tx` exists, we find it by obtaining a read lock and sorting the
priority queue index. Once sorted, we find the first `Tx` with lower priority and
size such that the new `Tx` would fit within the mempool's size limit. We then
remove this `Tx` from the priority queue index as well as the `<sender:*Tx>`
mapping.

This will require additional `O(n)` space and `O(n*log(n))` runtime complexity.

### Gossiping

When gossiping new transactions to peers, we first obtain a read lock on the
priority queue index. If we have not gossiped the top priority `Tx` for the
given peer, we return this `Tx` as the next transaction to gossip. Otherwise,
we sort the priority queue index and traverse in descending priority order until
we find the first non-sent `Tx` to the peer.

In order to facilitate this, we must also keep an ephemeral cache, mapping from
`sender` to a list of unique `Tx` identifiers (e.g. the `Tx` hash). A `Tx`
identifier is removed from this cache once it is either committed in a block or
gossiped to a peer. Note, if the `Tx` is committed in a block, this requires that
we iterates over the entire cache, i.e. each peer's list.

## Consequences

### Positive

- Transactions are allowed to be prioritized by the application.
- Transactions are allowed to be gossiped by priority.
- Allows future replace-by-priority functionality.

### Negative

- Additional bytes sent over the wire due to new fields added to `ResponseCheckTx`.
- Possible decreased throughput performance due to the necessity of various read
  and write locks, e.g. sorting when either gossiping or evicting transactions.
- It is possible that certain transactions broadcasted in a particular order may
  pass `CheckTx` but not end up being committed in a block because they fail
  `CheckTx` later. e.g. Consider Tx<sub>1</sub> that sends funds from existing
  account Alice to a _new_ account Bob with priority P<sub>1</sub> and then later
  Bob's _new_ account sends funds back to Alice in Tx<sub>2</sub> with P<sub>2</sub>,
  such that P<sub>2</sub> > P<sub>1</sub>. If executed in this order, both
  transactions will pass `CheckTx`. However, when a proposer is ready to select
  transactions for the next block proposal, they will select Tx<sub>2</sub> before
  Tx<sub>1</sub> and thus Tx<sub>2</sub> will _fail_ because Tx<sub>1</sub> must
  be executed first. These types of situations should be rare and can be
  circumvented by simply trying again at a later point in time or by ensuring the
  "child" priority is lower than the "parent" priority.

### Neutral

## References

- [ABCI++](https://github.com/tendermint/spec/blob/master/rfc/004-abci%2B%2B.md)
- [Mempool Discussion](https://github.com/tendermint/tendermint/discussions/6295)
