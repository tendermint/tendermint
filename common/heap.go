package common

import (
	"container/heap"
)

type Heap struct {
	pq priorityQueue
}

func NewHeap() *Heap {
	return &Heap{pq: make([]*pqItem, 0)}
}

func (h *Heap) Len() int {
	return len(h.pq)
}

func (h *Heap) Push(value interface{}, priority int) {
	heap.Push(&h.pq, &pqItem{value: value, priority: priority})
}

func (h *Heap) Pop() interface{} {
	item := heap.Pop(&h.pq).(*pqItem)
	return item.value
}

/*
func main() {
    h := NewHeap()

    h.Push(String("msg1"), 1)
    h.Push(String("msg3"), 3)
    h.Push(String("msg2"), 2)

    fmt.Println(h.Pop())
    fmt.Println(h.Pop())
    fmt.Println(h.Pop())
}
*/

///////////////////////
// From: http://golang.org/pkg/container/heap/#example__priorityQueue

type pqItem struct {
	value    interface{}
	priority int
	index    int
}

type priorityQueue []*pqItem

func (pq priorityQueue) Len() int { return len(pq) }

func (pq priorityQueue) Less(i, j int) bool {
	return pq[i].priority < pq[j].priority
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

func (pq *priorityQueue) Update(item *pqItem, value interface{}, priority int) {
	heap.Remove(pq, item.index)
	item.value = value
	item.priority = priority
	heap.Push(pq, item)
}
