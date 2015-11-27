package common

/*
The purpose of CList is to provide a goroutine-safe linked-list.
This list can be traversed concurrently by any number of goroutines.
However, removed CElements cannot be added back.
NOTE: Not all methods of container/list are (yet) implemented.
*/

import (
	"sync"
	"sync/atomic"
	"unsafe"
)

// CElement is an element of a linked-list
// Traversal from a CElement are goroutine-safe.
type CElement struct {
	next    unsafe.Pointer
	nextWg  *sync.WaitGroup
	prev    unsafe.Pointer
	prevWg  *sync.WaitGroup
	removed uint32
	Value   interface{}
}

// Blocking implementation of Next().
// May return nil iff CElement was tail and got removed.
func (e *CElement) NextWait() *CElement {
	for {
		e.nextWg.Wait()
		next := e.Next()
		if next == nil {
			if e.Removed() {
				return nil
			} else {
				continue
			}
		} else {
			return next
		}
	}
}

// Blocking implementation of Prev().
// May return nil iff CElement was head and got removed.
func (e *CElement) PrevWait() *CElement {
	for {
		e.prevWg.Wait()
		prev := e.Prev()
		if prev == nil {
			if e.Removed() {
				return nil
			} else {
				continue
			}
		} else {
			return prev
		}
	}
}

// Nonblocking, may return nil if at the end.
func (e *CElement) Next() *CElement {
	return (*CElement)(atomic.LoadPointer(&e.next))
}

// Nonblocking, may return nil if at the end.
func (e *CElement) Prev() *CElement {
	return (*CElement)(atomic.LoadPointer(&e.prev))
}

func (e *CElement) Removed() bool {
	return atomic.LoadUint32(&(e.removed)) > 0
}

func (e *CElement) setNextAtomic(next *CElement) {
	oldNext := (*CElement)(atomic.LoadPointer(&e.next))
	if next == nil && oldNext != nil {
		e.nextWg.Add(1) // NOTE: There is still a race condition for waiters, so we for-loop.
	}
	atomic.StorePointer(&(e.next), unsafe.Pointer(next))
	if next != nil && oldNext == nil {
		e.nextWg.Done()
	}
}

func (e *CElement) setPrevAtomic(prev *CElement) {
	oldPrev := (*CElement)(atomic.LoadPointer(&e.prev))
	if prev == nil && oldPrev != nil {
		e.prevWg.Add(1) // NOTE: There is still a race condition for waiters, so we for-loop.
	}
	atomic.StorePointer(&(e.prev), unsafe.Pointer(prev))
	if prev != nil && oldPrev == nil {
		e.prevWg.Done()
	}
}

func (e *CElement) setRemovedAtomic() {
	atomic.StoreUint32(&(e.removed), 1)
}

// CList represents a linked list.
// The zero value for CList is an empty list ready to use.
// Operations are goroutine-safe.
type CList struct {
	mtx  sync.Mutex
	wg   *sync.WaitGroup
	head *CElement // first element
	tail *CElement // last element
	len  int       // list length
}

func (l *CList) Init() *CList {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	l.wg = waitGroup1()
	l.head = nil
	l.tail = nil
	l.len = 0
	return l
}

func NewCList() *CList { return new(CList).Init() }

func (l *CList) Len() int {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	return l.len
}

func (l *CList) Front() *CElement {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	return l.head
}

func (l *CList) FrontWait() *CElement {
	for {
		l.mtx.Lock()
		head := l.head
		wg := l.wg
		l.mtx.Unlock()
		if head == nil {
			wg.Wait()
		} else {
			return head
		}
	}
}

func (l *CList) Back() *CElement {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	return l.tail
}

func (l *CList) BackWait() *CElement {
	for {
		l.mtx.Lock()
		tail := l.tail
		wg := l.wg
		l.mtx.Unlock()
		if tail == nil {
			wg.Wait()
		} else {
			return tail
		}
	}
}

func (l *CList) PushBack(v interface{}) *CElement {
	l.mtx.Lock()
	defer l.mtx.Unlock()

	// Construct a new element
	e := &CElement{
		prev:   nil,
		prevWg: waitGroup1(),
		next:   nil,
		nextWg: waitGroup1(),
		Value:  v,
	}

	// Release waiters on FrontWait/BackWait maybe
	if l.len == 0 {
		l.wg.Done()
	}
	l.len += 1

	// Modify the tail
	if l.tail == nil {
		l.head = e
		l.tail = e
	} else {
		l.tail.setNextAtomic(e)
		e.setPrevAtomic(l.tail)
		l.tail = e
	}

	return e
}

// NOTE: As per the contract of CList, removed elements cannot be added back.
func (l *CList) Remove(e *CElement) interface{} {
	l.mtx.Lock()
	defer l.mtx.Unlock()

	prev := e.Prev()
	next := e.Next()

	if l.head == nil || l.tail == nil {
		PanicSanity("Remove(e) on empty CList")
	}
	if prev == nil && l.head != e {
		PanicSanity("Remove(e) with false head")
	}
	if next == nil && l.tail != e {
		PanicSanity("Remove(e) with false tail")
	}

	// If we're removing the only item, make CList FrontWait/BackWait wait.
	if l.len == 1 {
		l.wg.Add(1)
	}
	l.len -= 1

	// Modify e.prev and e.next and connect
	if prev != nil {
		prev.setNextAtomic(next)
	}
	if next != nil {
		next.setPrevAtomic(prev)
	}

	// Mark e as removed so NextWait/PrevWait can return.
	e.setRemovedAtomic()

	// Set .Done() on e, otherwise waiters will wait forever.
	if prev == nil {
		e.prevWg.Done()
	}
	if next == nil {
		e.nextWg.Done()
	}

	return e.Value
}

func waitGroup1() (wg *sync.WaitGroup) {
	wg = &sync.WaitGroup{}
	wg.Add(1)
	return
}
