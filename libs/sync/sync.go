package sync

/*
// For detecting deadlock situations

import deadlock "github.com/sasha-s/go-deadlock"
import "sync"

type Mutex struct {
	deadlock.Mutex
}

type RWMutex struct {
	deadlock.RWMutex
}

type WaitGroup struct {
	sync.WaitGroup
}
*/

import "sync"

type Mutex struct {
	sync.Mutex
}

type RWMutex struct {
	sync.RWMutex
}
type WaitGroup struct {
	sync.WaitGroup
}
