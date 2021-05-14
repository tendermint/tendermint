package mempool

import (
	"container/list"

	tmsync "github.com/tendermint/tendermint/libs/sync"
	"github.com/tendermint/tendermint/types"
)

// TxCache defines an interface for raw transaction caching.
type TxCache interface {
	Reset()
	Push(tx types.Tx) bool
	Remove(tx types.Tx)
}

var _ TxCache = (*LRUTxCache)(nil)

// LRUTxCache maintains a thread-safe LRU cache of raw transactions. The cache
// only stores the hash of the raw transaction.
type LRUTxCache struct {
	mtx      tmsync.Mutex
	size     int
	cacheMap map[[TxKeySize]byte]*list.Element
	list     *list.List
}

func NewLRUTxCache(cacheSize int) *LRUTxCache {
	return &LRUTxCache{
		size:     cacheSize,
		cacheMap: make(map[[TxKeySize]byte]*list.Element, cacheSize),
		list:     list.New(),
	}
}

// GetList returns the underlying linked-list that backs the LRU cache. Note,
// this should be used for testing purposes only!
func (c *LRUTxCache) GetList() *list.List {
	return c.list
}

// Reset resets the cache to an empty state.
func (c *LRUTxCache) Reset() {
	c.mtx.Lock()
	c.cacheMap = make(map[[TxKeySize]byte]*list.Element, c.size)
	c.list.Init()
	c.mtx.Unlock()
}

// Push adds the given raw transaction to the cache and returns true if it was
// newly added. Otherwise, it returns false.
func (c *LRUTxCache) Push(tx types.Tx) bool {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	key := TxKey(tx)
	if moved, exists := c.cacheMap[key]; exists {
		c.list.MoveToBack(moved)
		return false
	}

	if c.list.Len() >= c.size {
		front := c.list.Front()
		if front != nil {
			frontKey := front.Value.([TxKeySize]byte)
			delete(c.cacheMap, frontKey)
			c.list.Remove(front)
		}
	}

	e := c.list.PushBack(key)
	c.cacheMap[key] = e
	return true
}

// Remove removes the given raw transaction from the cache.
func (c *LRUTxCache) Remove(tx types.Tx) {
	c.mtx.Lock()

	key := TxKey(tx)
	popped := c.cacheMap[key]
	delete(c.cacheMap, key)
	if popped != nil {
		c.list.Remove(popped)
	}

	c.mtx.Unlock()
}

// NopTxCache defines a no-op raw transaction cache.
type NopTxCache struct{}

var _ TxCache = (*NopTxCache)(nil)

func (NopTxCache) Reset()             {}
func (NopTxCache) Push(types.Tx) bool { return true }
func (NopTxCache) Remove(types.Tx)    {}
