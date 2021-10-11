package sync

import (
	"sync"
)

// RWInitMutex is a RWMutex that also has the notion of an initialized state
// that writers can set, and init-readers can wait for.  Regular readers block
// while writers hold the lock, but do not block for initialization.
type RWInitMutex struct {
	*RWMutex
	writeLocked bool
	initialized bool
	waitForInit *sync.Cond
}

func NewRWInitMutex() *RWInitMutex {
	mtx := new(RWMutex)

	// Condition variables should work with read locks, as long as the actual
	// state update is done under the associated write lock to provide
	// happens-before visibility. See comment for notifyListAdd() in
	// src/runtime/sema.go in the Go sources
	waitForInit := sync.NewCond(mtx.RLocker())
	rwi := &RWInitMutex{
		RWMutex:     mtx,
		waitForInit: waitForInit,
	}

	return rwi
}

// RInitLock waits until initialized, then takes a read lock.
func (rwi *RWInitMutex) RInitLock() {
	rwi.waitForInit.L.Lock()
	// We need to wait for the initialized state.
	for !rwi.initialized {
		rwi.waitForInit.Wait()
	}
}

// RInitUnlock unlocks the read lock taken by RInitLock.
func (rwi *RWInitMutex) RInitUnlock() {
	rwi.waitForInit.L.Unlock()
}

// rInitLocker provides a curried form of the RInitLock/RInitUnlock functions.
type rInitLocker RWInitMutex

var _ sync.Locker = &rInitLocker{}

func (r *rInitLocker) Lock()   { (*RWInitMutex)(r).RInitLock() }
func (r *rInitLocker) Unlock() { (*RWInitMutex)(r).RInitUnlock() }

// Lock instruments the mutex write lock.
func (rwi *RWInitMutex) Lock() {
	rwi.RWMutex.Lock()
	rwi.writeLocked = true
}

// Unlock instruments the mutex write lock.
func (rwi *RWInitMutex) Unlock() {
	if !rwi.writeLocked {
		panic("Must write-lock the RWInitMutex to call Unlock")
	}
	rwi.writeLocked = false
	rwi.RWMutex.Unlock()
}

// Initialize marks the RWInitMutex as initialized and awakens all goroutines which were
// waiting for initialization. The write lock must be held when calling Initialize().
// This is a separate method to allow write-locking without initialization.
func (rwi *RWInitMutex) Initialize() {
	if !rwi.writeLocked {
		panic("Must write-lock the RWInitMutex to call Initialize")
	}

	if rwi.initialized {
		// Nothing else to do.
		return
	}

	// Advertise our initialized status.
	rwi.initialized = true
	rwi.waitForInit.Broadcast()
}
