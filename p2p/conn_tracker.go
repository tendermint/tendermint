package p2p

import (
	"fmt"
	"net"
	"sync"
	"time"
)

type connectionTracker interface {
	AddConn(net.IP) error
	CheckConn(net.IP) (int, bool)
	RemoveConn(net.IP)
}

type connTrackerImpl struct {
	cache       map[string]uint
	lastConnect map[string]time.Time
	mutex       sync.RWMutex
	max         uint
	window      time.Duration
}

func newConnTracker(max uint, window time.Duration) connectionTracker {
	return &connTrackerImpl{
		cache:       make(map[string]uint),
		lastConnect: make(map[string]time.Time),
		max:         max,
	}
}

func (rat *connTrackerImpl) AddConn(addr net.IP) error {
	address := addr.String()
	rat.mutex.Lock()
	defer rat.mutex.Unlock()

	if num := rat.cache[address]; num >= rat.max {
		return fmt.Errorf("%q has %d connections [max=%d]", address, num, rat.max)
	} else if num == 0 {
		// if there is already at least connection, check to
		// see if it was established before within the window,
		// and error if so.
		if last := rat.lastConnect[address]; time.Since(last) < rat.window {
			return fmt.Errorf("%q tried to connect within window of last %s", address, rat.window)
		}
	}

	rat.cache[address] += 1
	rat.lastConnect[address] = time.Now()

	return nil
}

func (rat *connTrackerImpl) CheckConn(addr net.IP) (int, bool) {
	rat.mutex.RLock()
	defer rat.mutex.RUnlock()

	num := rat.cache[addr.String()]

	return int(num), num >= rat.max
}

func (rat *connTrackerImpl) RemoveConn(addr net.IP) {
	address := addr.String()
	rat.mutex.Lock()
	defer rat.mutex.Unlock()

	if num := rat.cache[address]; num > 0 {
		rat.cache[address] -= 1
	}
	if last, ok := rat.lastConnect[address]; ok && time.Since(last) > rat.window {
		delete(rat.lastConnect, address)
	}

}
