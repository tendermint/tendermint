package sync

import "sync/atomic"

// AtomicBool is an atomic Boolean.
// Its methods are all atomic, thus safe to be called by multiple goroutines simultaneously.
// Note: When embedding into a struct one should always use *AtomicBool to avoid copy.
// it's a simple implmentation from https://github.com/tevino/abool
type AtomicBool int32

// NewBool creates an AtomicBool with given default value.
func NewBool(ok bool) *AtomicBool {
	ab := new(AtomicBool)
	if ok {
		ab.Set()
	}
	return ab
}

// Set sets the Boolean to true.
func (ab *AtomicBool) Set() {
	atomic.StoreInt32((*int32)(ab), 1)
}

// UnSet sets the Boolean to false.
func (ab *AtomicBool) UnSet() {
	atomic.StoreInt32((*int32)(ab), 0)
}

// IsSet returns whether the Boolean is true.
func (ab *AtomicBool) IsSet() bool {
	return atomic.LoadInt32((*int32)(ab))&1 == 1
}
