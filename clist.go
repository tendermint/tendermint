package common

/*
The purpose of CList is to provide a goroutine-safe linked-list.
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
	next  unsafe.Pointer
	wg    *sync.WaitGroup
	Value interface{}
}

// Blocking implementation of Next().
// If return is nil, this element was removed from the list.
func (e *CElement) NextWait() *CElement {
	e.wg.Wait()
	return e.Next()
}

func (e *CElement) Next() *CElement {
	next := atomic.LoadPointer(&e.next)
	if next == nil {
		return nil
	}
	return (*CElement)(next)
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
	e := &CElement{
		next:  nil,
		wg:    waitGroup1(),
		Value: v,
	}
	l.mtx.Lock()
	defer l.mtx.Unlock()
	l.len += 1
	if l.tail == nil {
		l.head = e
		l.tail = e
		l.wg.Done()
		return e
	} else {
		oldTail := l.tail
		atomic.StorePointer(&oldTail.next, unsafe.Pointer(e))
		l.tail = e
		oldTail.wg.Done()
		return e
	}
	return e
}

func (l *CList) RemoveFront() interface{} {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	if l.head == nil {
		return nil
	}
	oldFront := l.head
	next := (*CElement)(oldFront.next)
	l.head = next
	if next == nil {
		l.tail = nil
		l.wg = waitGroup1()
	}
	l.len -= 1
	atomic.StorePointer(&oldFront.next, unsafe.Pointer(nil))
	return oldFront.Value
}

func waitGroup1() (wg *sync.WaitGroup) {
	wg = &sync.WaitGroup{}
	wg.Add(1)
	return
}
