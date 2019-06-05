// The mempool pushes new txs onto the proxyAppConn.
// It gets a stream of (req, res) tuples from the proxy.
// The mempool stores good txs in a concurrent linked-list.

// Multiple concurrent go-routines can traverse this linked-list
// safely by calling .NextWait() on each element.

// So we have several go-routines:
// 1. Consensus calling Update() and Reap() synchronously
// 2. Many mempool reactor's peer routines calling CheckTx()
// 3. Many mempool reactor's peer routines traversing the txs linked list
// 4. Another goroutine calling GarbageCollectTxs() periodically

// To manage these goroutines, there are three methods of locking.
// 1. Mutations to the linked-list is protected by an internal mtx (CList is goroutine-safe)
// 2. Mutations to the linked-list elements are atomic
// 3. CheckTx() calls can be paused upon Update() and Reap(), protected by .proxyMtx

// Garbage collection of old elements from mempool.txs is handlde via
// the DetachPrev() call, which makes old elements not reachable by
// peer broadcastTxRoutine() automatically garbage collected.

// TODO: Better handle abci client errors. (make it automatically handle connection errors)
package mempool
