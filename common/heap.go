package common

import (
	"bytes"
	"container/heap"
)

/*
	Example usage:

	```
	h := NewHeap()

	h.Push("msg1", 1)
	h.Push("msg3", 3)
	h.Push("msg2", 2)

	fmt.Println(h.Pop()) // msg1
	fmt.Println(h.Pop()) // msg2
	fmt.Println(h.Pop()) // msg3
	```
*/
type Heap struct {
	pq priorityQueue
}

func NewHeap() *Heap {
	return &Heap{pq: make([]*pqItem, 0)}
}

func (h *Heap) Len() int64 {
	return int64(len(h.pq))
}

func (h *Heap) Push(value interface{}, priority int) {
	heap.Push(&h.pq, &pqItem{value: value, priority: cmpInt(priority)})
}

func (h *Heap) PushBytes(value interface{}, priority []byte) {
	heap.Push(&h.pq, &pqItem{value: value, priority: cmpBytes(priority)})
}

func (h *Heap) PushComparable(value interface{}, priority Comparable) {
	heap.Push(&h.pq, &pqItem{value: value, priority: priority})
}

func (h *Heap) Peek() interface{} {
	if len(h.pq) == 0 {
		return nil
	}
	return h.pq[0].value
}

func (h *Heap) Update(value interface{}, priority Comparable) {
	h.pq.Update(h.pq[0], value, priority)
}

func (h *Heap) Pop() interface{} {
	item := heap.Pop(&h.pq).(*pqItem)
	return item.value
}

//-----------------------------------------------------------------------------
// From: http://golang.org/pkg/container/heap/#example__priorityQueue

type pqItem struct {
	value    interface{}
	priority Comparable
	index    int
}

type priorityQueue []*pqItem

func (pq priorityQueue) Len() int { return len(pq) }

func (pq priorityQueue) Less(i, j int) bool {
	return pq[i].priority.Less(pq[j].priority)
}

func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *priorityQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*pqItem)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *priorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	item.index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}

func (pq *priorityQueue) Update(item *pqItem, value interface{}, priority Comparable) {
	item.value = value
	item.priority = priority
	heap.Fix(pq, item.index)
}

//--------------------------------------------------------------------------------
// Comparable

type Comparable interface {
	Less(o interface{}) bool
}

type cmpInt int

func (i cmpInt) Less(o interface{}) bool {
	return int(i) < int(o.(cmpInt))
}

type cmpBytes []byte

func (bz cmpBytes) Less(o interface{}) bool {
	return bytes.Compare([]byte(bz), []byte(o.(cmpBytes))) < 0
}
