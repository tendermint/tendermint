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
	"sync"
)

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
	mtx        sync.RWMutex
	prev       *CElement
	prevWg     *sync.WaitGroup
	prevWaitCh chan struct{}
	next       *CElement
	nextWg     *sync.WaitGroup
	nextWaitCh chan struct{}
	removed    bool

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

// PrevWaitChan can be used to wait until Prev becomes not nil. Once it does,
// channel will be closed.
func (e *CElement) PrevWaitChan() <-chan struct{} {
	e.mtx.RLock()
	defer e.mtx.RUnlock()

	return e.prevWaitCh
}

// NextWaitChan can be used to wait until Next becomes not nil. Once it does,
// channel will be closed.
func (e *CElement) NextWaitChan() <-chan struct{} {
	e.mtx.RLock()
	defer e.mtx.RUnlock()

	return e.nextWaitCh
}

// Nonblocking, may return nil if at the end.
func (e *CElement) Next() *CElement {
	e.mtx.RLock()
	defer e.mtx.RUnlock()

	return e.next
}

// Nonblocking, may return nil if at the end.
func (e *CElement) Prev() *CElement {
	e.mtx.RLock()
	defer e.mtx.RUnlock()

	return e.prev
}

func (e *CElement) Removed() bool {
	e.mtx.RLock()
	defer e.mtx.RUnlock()

	return e.removed
}

func (e *CElement) DetachNext() {
	if !e.Removed() {
		panic("DetachNext() must be called after Remove(e)")
	}
	e.mtx.Lock()
	defer e.mtx.Unlock()

	e.next = nil
}

func (e *CElement) DetachPrev() {
	if !e.Removed() {
		panic("DetachPrev() must be called after Remove(e)")
	}
	e.mtx.Lock()
	defer e.mtx.Unlock()

	e.prev = nil
}

// NOTE: This function needs to be safe for
// concurrent goroutines waiting on nextWg.
func (e *CElement) SetNext(newNext *CElement) {
	e.mtx.Lock()
	defer e.mtx.Unlock()

	oldNext := e.next
	e.next = newNext
	if oldNext != nil && newNext == nil {
		// See https://golang.org/pkg/sync/:
		//
		// If a WaitGroup is reused to wait for several independent sets of
		// events, new Add calls must happen after all previous Wait calls have
		// returned.
		e.nextWg = waitGroup1() // WaitGroups are difficult to re-use.
		e.nextWaitCh = make(chan struct{})
	}
	if oldNext == nil && newNext != nil {
		e.nextWg.Done()
		close(e.nextWaitCh)
	}
}

// NOTE: This function needs to be safe for
// concurrent goroutines waiting on prevWg
func (e *CElement) SetPrev(newPrev *CElement) {
	e.mtx.Lock()
	defer e.mtx.Unlock()

	oldPrev := e.prev
	e.prev = newPrev
	if oldPrev != nil && newPrev == nil {
		e.prevWg = waitGroup1() // WaitGroups are difficult to re-use.
		e.prevWaitCh = make(chan struct{})
	}
	if oldPrev == nil && newPrev != nil {
		e.prevWg.Done()
		close(e.prevWaitCh)
	}
}

func (e *CElement) SetRemoved() {
	e.mtx.Lock()
	defer e.mtx.Unlock()

	e.removed = true

	// This wakes up anyone waiting in either direction.
	if e.prev == nil {
		e.prevWg.Done()
		close(e.prevWaitCh)
	}
	if e.next == nil {
		e.nextWg.Done()
		close(e.nextWaitCh)
	}
}

//--------------------------------------------------------------------------------

// CList represents a linked list.
// The zero value for CList is an empty list ready to use.
// Operations are goroutine-safe.
type CList struct {
	mtx    sync.RWMutex
	wg     *sync.WaitGroup
	waitCh chan struct{}
	head   *CElement // first element
	tail   *CElement // last element
	len    int       // list length
}

func (l *CList) Init() *CList {
	l.mtx.Lock()
	defer l.mtx.Unlock()

	l.wg = waitGroup1()
	l.waitCh = make(chan struct{})
	l.head = nil
	l.tail = nil
	l.len = 0
	return l
}

func New() *CList { return new(CList).Init() }

func (l *CList) Len() int {
	l.mtx.RLock()
	defer l.mtx.RUnlock()

	return l.len
}

func (l *CList) Front() *CElement {
	l.mtx.RLock()
	defer l.mtx.RUnlock()

	return l.head
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
	defer l.mtx.RUnlock()

	return l.tail
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
	l.mtx.Lock()
	defer l.mtx.Unlock()

	return l.waitCh
}

func (l *CList) PushBack(v interface{}) *CElement {
	l.mtx.Lock()
	defer l.mtx.Unlock()

	// Construct a new element
	e := &CElement{
		prev:       nil,
		prevWg:     waitGroup1(),
		prevWaitCh: make(chan struct{}),
		next:       nil,
		nextWg:     waitGroup1(),
		nextWaitCh: make(chan struct{}),
		removed:    false,
		Value:      v,
	}

	// Release waiters on FrontWait/BackWait maybe
	if l.len == 0 {
		l.wg.Done()
		close(l.waitCh)
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

	return e
}

// CONTRACT: Caller must call e.DetachPrev() and/or e.DetachNext() to avoid memory leaks.
// NOTE: As per the contract of CList, removed elements cannot be added back.
func (l *CList) Remove(e *CElement) interface{} {
	l.mtx.Lock()
	defer l.mtx.Unlock()

	prev := e.Prev()
	next := e.Next()

	if l.head == nil || l.tail == nil {
		panic("Remove(e) on empty CList")
	}
	if prev == nil && l.head != e {
		panic("Remove(e) with false head")
	}
	if next == nil && l.tail != e {
		panic("Remove(e) with false tail")
	}

	// If we're removing the only item, make CList FrontWait/BackWait wait.
	if l.len == 1 {
		l.wg = waitGroup1() // WaitGroups are difficult to re-use.
		l.waitCh = make(chan struct{})
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

	return e.Value
}

func waitGroup1() (wg *sync.WaitGroup) {
	wg = &sync.WaitGroup{}
	wg.Add(1)
	return
}
