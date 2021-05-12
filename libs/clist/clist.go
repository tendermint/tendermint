package clist

/*

The purpose of CList is to provide a goroutine-safe linked-list.
This list can be traversed concurrently by any number of goroutines.
However, removed CElements cannot be added back.
NOTE: Not all methods of container/list are (yet) implemented.
NOTE: Removed elements need to DetachPrev or DetachNext consistently
to ensure garbage collection of removed elements.

*/

import (
	"fmt"
	"sync"

	tmsync "github.com/tendermint/tendermint/libs/sync"
)

// MaxLength is the max allowed number of elements a linked list is
// allowed to contain.
// If more elements are pushed to the list it will panic.
const MaxLength = int(^uint(0) >> 1)

/*

CElement is an element of a linked-list
Traversal from a CElement is goroutine-safe.

We can't avoid using WaitGroups or for-loops given the documentation
spec without re-implementing the primitives that already exist in
golang/sync. Notice that WaitGroup allows many go-routines to be
simultaneously released, which is what we want. Mutex doesn't do
this. RWMutex does this, but it's clumsy to use in the way that a
WaitGroup would be used -- and we'd end up having two RWMutex's for
prev/next each, which is doubly confusing.

sync.Cond would be sort-of useful, but we don't need a write-lock in
the for-loop. Use sync.Cond when you need serial access to the
"condition". In our case our condition is if `next != nil || removed`,
and there's no reason to serialize that condition for goroutines
waiting on NextWait() (since it's just a read operation).

*/
type CElement struct {
	mtx     tmsync.RWMutex
	prev    *CElement
	prevWg  *sync.WaitGroup
	next    *CElement
	nextWg  *sync.WaitGroup
	removed bool

	Value interface{} // immutable
}

// Blocking implementation of Next().
// May return nil iff CElement was tail and got removed.
func (e *CElement) NextWait() *CElement {
	for {
		e.mtx.RLock()
		next := e.next
		nextWg := e.nextWg
		removed := e.removed
		e.mtx.RUnlock()

		if next != nil || removed {
			return next
		}

		nextWg.Wait()
		// e.next doesn't necessarily exist here.
		// That's why we need to continue a for-loop.
	}
}

// Blocking implementation of Prev().
// May return nil iff CElement was head and got removed.
func (e *CElement) PrevWait() *CElement {
	for {
		e.mtx.RLock()
		prev := e.prev
		prevWg := e.prevWg
		removed := e.removed
		e.mtx.RUnlock()

		if prev != nil || removed {
			return prev
		}

		prevWg.Wait()
	}
}

// NextWaitChan can be used to wait until Next becomes not nil. Once it does,
// channel will be closed.
func (e *CElement) NextWaitChan() <-chan struct{} {
	e.mtx.RLock()
	defer e.mtx.RUnlock()
	wg := e.nextWg

	ret := make(chan struct{})
	go func() {
		wg.Wait()
		close(ret)
	}()

	return ret
}

// PrevWaitChan can be used to wait until Prev becomes not nil. Once it does,
// channel will be closed.
func (e *CElement) PrevWaitChan() <-chan struct{} {
	e.mtx.RLock()
	defer e.mtx.RUnlock()
	wg := e.prevWg

	ret := make(chan struct{})
	go func() {
		wg.Wait()
		close(ret)
	}()

	return ret
}

// Nonblocking, may return nil if at the end.
func (e *CElement) Next() *CElement {
	e.mtx.RLock()
	val := e.next
	e.mtx.RUnlock()
	return val
}

// Nonblocking, may return nil if at the end.
func (e *CElement) Prev() *CElement {
	e.mtx.RLock()
	prev := e.prev
	e.mtx.RUnlock()
	return prev
}

func (e *CElement) DetachPrev() {
	e.mtx.Lock()
	if !e.removed {
		e.mtx.Unlock()
		panic("DetachPrev() must be called after Remove(e)")
	}
	e.prev = nil
	e.mtx.Unlock()
}

// NOTE: This function needs to be safe for
// concurrent goroutines waiting on nextWg.
func (e *CElement) SetNext(newNext *CElement) {
	e.mtx.Lock()

	oldNext := e.next
	e.next = newNext
	if oldNext != nil && newNext == nil {
		// See https://golang.org/pkg/sync/:
		//
		// If a WaitGroup is reused to wait for several independent sets of
		// events, new Add calls must happen after all previous Wait calls have
		// returned.
		e.nextWg = waitGroup1() // WaitGroups are difficult to re-use.
	}
	if oldNext == nil && newNext != nil {
		e.nextWg.Done()
	}
	e.mtx.Unlock()
}

// NOTE: This function needs to be safe for
// concurrent goroutines waiting on prevWg
func (e *CElement) SetPrev(newPrev *CElement) {
	e.mtx.Lock()

	oldPrev := e.prev
	e.prev = newPrev
	if oldPrev != nil && newPrev == nil {
		e.prevWg = waitGroup1() // WaitGroups are difficult to re-use.
	}
	if oldPrev == nil && newPrev != nil {
		e.prevWg.Done()
	}
	e.mtx.Unlock()
}

func (e *CElement) SetRemoved() {
	e.mtx.Lock()

	e.removed = true

	// This wakes up anyone waiting in either direction.
	if e.prev == nil {
		e.prevWg.Done()
	}
	if e.next == nil {
		e.nextWg.Done()
	}
	e.mtx.Unlock()
}

//--------------------------------------------------------------------------------

// CList represents a linked list.
// The zero value for CList is an empty list ready to use.
// Operations are goroutine-safe.
// Panics if length grows beyond the max.
type CList struct {
	mtx    tmsync.RWMutex
	wg     *sync.WaitGroup
	head   *CElement // first element
	tail   *CElement // last element
	len    int       // list length
	maxLen int       // max list length
}

// Return CList with MaxLength. CList will panic if it goes beyond MaxLength.
func New() *CList { return newWithMax(MaxLength) }

// Return CList with given maxLength.
// Will panic if list exceeds given maxLength.
func newWithMax(maxLength int) *CList {
	l := new(CList)
	l.maxLen = maxLength

	l.wg = waitGroup1()
	l.head = nil
	l.tail = nil
	l.len = 0

	return l
}

func (l *CList) Len() int {
	l.mtx.RLock()
	len := l.len
	l.mtx.RUnlock()
	return len
}

func (l *CList) Front() *CElement {
	l.mtx.RLock()
	head := l.head
	l.mtx.RUnlock()
	return head
}

func (l *CList) FrontWait() *CElement {
	// Loop until the head is non-nil else wait and try again
	for {
		l.mtx.RLock()
		head := l.head
		wg := l.wg
		l.mtx.RUnlock()

		if head != nil {
			return head
		}
		wg.Wait()
		// NOTE: If you think l.head exists here, think harder.
	}
}

func (l *CList) Back() *CElement {
	l.mtx.RLock()
	back := l.tail
	l.mtx.RUnlock()
	return back
}

func (l *CList) BackWait() *CElement {
	for {
		l.mtx.RLock()
		tail := l.tail
		wg := l.wg
		l.mtx.RUnlock()

		if tail != nil {
			return tail
		}
		wg.Wait()
		// l.tail doesn't necessarily exist here.
		// That's why we need to continue a for-loop.
	}
}

// WaitChan can be used to wait until Front or Back becomes not nil. Once it
// does, channel will be closed.
func (l *CList) WaitChan() <-chan struct{} {
	l.mtx.RLock()
	defer l.mtx.RUnlock()
	wg := l.wg

	ret := make(chan struct{})
	go func() {
		wg.Wait()
		close(ret)
	}()

	return ret
}

// Panics if list grows beyond its max length.
func (l *CList) PushBack(v interface{}) *CElement {
	l.mtx.Lock()

	// Construct a new element
	e := &CElement{
		prev:    nil,
		prevWg:  waitGroup1(),
		next:    nil,
		nextWg:  waitGroup1(),
		removed: false,
		Value:   v,
	}

	// Release waiters on FrontWait/BackWait maybe
	if l.len == 0 {
		l.wg.Done()
	}
	if l.len >= l.maxLen {
		panic(fmt.Sprintf("clist: maximum length list reached %d", l.maxLen))
	}
	l.len++

	// Modify the tail
	if l.tail == nil {
		l.head = e
		l.tail = e
	} else {
		e.SetPrev(l.tail) // We must init e first.
		l.tail.SetNext(e) // This will make e accessible.
		l.tail = e        // Update the list.
	}
	l.mtx.Unlock()
	return e
}

// CONTRACT: Caller must call e.DetachPrev() and/or e.DetachNext() to avoid memory leaks.
// NOTE: As per the contract of CList, removed elements cannot be added back.
func (l *CList) Remove(e *CElement) interface{} {
	l.mtx.Lock()

	prev := e.Prev()
	next := e.Next()

	if l.head == nil || l.tail == nil {
		l.mtx.Unlock()
		panic("Remove(e) on empty CList")
	}
	if prev == nil && l.head != e {
		l.mtx.Unlock()
		panic("Remove(e) with false head")
	}
	if next == nil && l.tail != e {
		l.mtx.Unlock()
		panic("Remove(e) with false tail")
	}

	// If we're removing the only item, make CList FrontWait/BackWait wait.
	if l.len == 1 {
		l.wg = waitGroup1() // WaitGroups are difficult to re-use.
	}

	// Update l.len
	l.len--

	// Connect next/prev and set head/tail
	if prev == nil {
		l.head = next
	} else {
		prev.SetNext(next)
	}
	if next == nil {
		l.tail = prev
	} else {
		next.SetPrev(prev)
	}

	// Set .Done() on e, otherwise waiters will wait forever.
	e.SetRemoved()

	l.mtx.Unlock()
	return e.Value
}

func waitGroup1() (wg *sync.WaitGroup) {
	wg = &sync.WaitGroup{}
	wg.Add(1)
	return
}
