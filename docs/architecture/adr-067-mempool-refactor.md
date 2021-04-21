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
prioritize transactions in a flexible and performant manner.

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
we will introduce new fields, `priority`, `sender`, and `nonce` , into the
`ResponseCheckTx` type and augment the existing mempool data structure to
facilitate prioritization of uncommitted transactions in addition to extended
functionality such as replace-by-priority and allowing multiple transactions to
exist from the same sender with varying priorities.

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
+ int64          nonce      = 11;
}
```

It is entirely up the application in determining how these fields are populated
and with what values, e.g. the `sender` could be the signer  and fee payer 
the transaction, the `priority` could be the cumulative sum of the fee(s), and
the `nonce` could be the signer's sequence number/nonce.

Only `sender` is required, while `priority` and `nonce` can be omitted which
would result in using the default value of zero.

### Mempool

The existing concurrent-safe linked-list will be removed entirely in favor of a
thread-safe map of `<sender:[]Tx>`, i.e a mapping from `sender` to a list of `Tx`
objects, where each `Tx` is ordered by monotonically increasing, but not
necessarily consecutive, `nonce` values. For example, the sender `Alice`, whose
current committed nonce value is `3`, could have transactions in this map with
nonce values `<4,5,6,8>`, where only the transaction with nonce `4` could be
processed next.

Note, the mempool itself does not and will not know about the latest committed
nonce value, but it is assumed that all transactions that end up in the mempool
pass `CheckTx`. In addition, the application and thus Tendermint's mempool does
not necessarily have to utilize the concept of a nonce.

On top of this mapping, we index all transactions by priority using a priority
queue. When a proposer is ready to reap transactions for the next block proposal,
transactions are selected from this priority index. For each priority-indexed
transaction, we evaluate if that transaction is at the head of the sender's
list of transactions, if so, it is included in the block and the index is removed
along with the transaction itself.
## Consequences

### Positive

- Transactions are allowed to be prioritized by the application
- Transactions can be replaced by priority

### Negative

- Additional bytes sent over the wire due to new fields added to `ResponseCheckTx`

### Neutral

## References

- [ABCI++](https://github.com/tendermint/spec/blob/master/rfc/004-abci%2B%2B.md)
- [mempool discussion](https://github.com/tendermint/tendermint/discussions/6295)
