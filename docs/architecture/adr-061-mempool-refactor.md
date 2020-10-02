# ADR 061: Generalize Mempool Implementation

## Changelog

- 2020-10-01: Initial version. (@ninjaahhh)

## Context

When designing a [priority-based mempool](https://github.com/tendermint/spec/pull/154), we realized the current mempool interface needs to be refactored to support different underlying data structures to be priority-aware (e.g. max-heap, balance tree, etc.). This document lists necessary changes on the instance (i.e. struct)definition.

## Decision

[TBD]

## Detailed Design

Existing mempool code has many places tightly coupled with `CList` implementation (like in mempool reactor), and this proposed change mainly works as a middle ground to abstract away this coupling to allow future mempool implementation using other data structures for better performances related to transaction priorities.

```golang
type basemempool struct {
    // Unified interface for actual mempool implementations
    mempoolImpl

    // Atomic integers
    height   int64 // the last block Update()'d to
    txsBytes int64 // total size of mempool, in bytes

    // Notify listeners (ie. consensus) when txs are available
    notifiedTxsAvailable bool
    txsAvailable         chan struct{} // fires once for each height, when the mempool is not empty

    // Keep a cache of already-seen txs.
    // This reduces the pressure on the proxyApp.
    cache     txCache
    preCheck  PreCheckFunc
    postCheck PostCheckFunc

    config *cfg.MempoolConfig

    // Exclusive mutex for Update method to prevent concurrent execution of
    // CheckTx or ReapMaxBytesMaxGas(ReapMaxTxs) methods.
    updateMtx sync.RWMutex

    wal          *auto.AutoFile // a log of mempool txs
    proxyAppConn proxy.AppConnMempool

    logger  log.Logger
    metrics *Metrics
}

type mempoolImpl interface {
    Size() int

    addTx(*mempoolTx, uint64)
    removeTx(types.Tx) bool // return whether corresponding element is removed or not
    updateRecheckCursor()
    reapMaxTxs(int) types.Txs
    reapMaxBytesMaxGas(int64, int64) types.Txs // based on priority
    recheckTxs(proxy.AppConnMempool)
    isRecheckCursorNil() bool
    getRecheckCursorTx() *mempoolTx
    getMempoolTx(types.Tx) *mempoolTx
    deleteAll()
    // ...and more
}
```

In this way, mempool's shared code will live under `basemempool` (callback handling, WAL, etc.) while different `mempoolImpl` only needs to implement required methods on transaction addition / removal / iteration etc. The proposed change is [implemented in this code base](https://github.com/QuarkChain/tendermintx/blob/master/mempool/mempool.go).

## Status

Proposed

## Consequences

### Positive

- A generalized and decoupled mempool will allow different implementations supporting various desired design features (such as priority-aware transaction handling)

### Negative

- Major refactoring needs to be done

## References

- [Libra mempool](https://developers.libra.org/docs/crates/mempool) employs a hash map of accounts plus a BTree for [`PriorityIndex`](https://github.com/libra/libra/blob/c5f6a2b4a6be63f6ef8f17a2c1cf192c9a23bb07/mempool/src/core_mempool/index.rs#L25-L27)
- [Introduction to ABCIx from QuarkChain](https://forum.cosmos.network/t/introduction-to-abcix-an-extension-of-abci-with-greater-flexibility-and-security/3771/)
- [On ABCIx’s priority-based mempool implementation](https://forum.cosmos.network/t/on-abcixs-priority-based-mempool-implementation/3912)
- [Existing ABCIx implementation](https://github.com/QuarkChain/tendermintx)
