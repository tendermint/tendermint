package v2

import (
	"github.com/Workiva/go-datastructures/queue"
)

// Event is the type that can be added to the priority queue.
type Event queue.Item

type priority interface {
	Compare(other queue.Item) int
	Priority() int
}

type priorityLow struct{}
type priorityNormal struct{}
type priorityHigh struct{}

func (p priorityLow) Priority() int {
	return 1
}

func (p priorityNormal) Priority() int {
	return 2
}

func (p priorityHigh) Priority() int {
	return 3
}

func (p priorityLow) Compare(other queue.Item) int {
	op := other.(priority)
	if p.Priority() > op.Priority() {
		return 1
	} else if p.Priority() == op.Priority() {
		return 0
	}
	return -1
}

func (p priorityNormal) Compare(other queue.Item) int {
	op := other.(priority)
	if p.Priority() > op.Priority() {
		return 1
	} else if p.Priority() == op.Priority() {
		return 0
	}
	return -1
}

func (p priorityHigh) Compare(other queue.Item) int {
	op := other.(priority)
	if p.Priority() > op.Priority() {
		return 1
	} else if p.Priority() == op.Priority() {
		return 0
	}
	return -1
}

type noOpEvent struct {
	priorityLow
}

var noOp = noOpEvent{}
