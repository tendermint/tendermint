package common

import (
	"sync"
	"testing"
	"time"

	// make govet noshadow happy...
	asrt "github.com/stretchr/testify/assert"
)

type rCounter struct {
	input chan time.Time
	mtx   sync.Mutex
	count int
}

func (c *rCounter) Increment() {
	c.mtx.Lock()
	c.count++
	c.mtx.Unlock()
}

func (c *rCounter) Count() int {
	c.mtx.Lock()
	val := c.count
	c.mtx.Unlock()
	return val
}

// Read should run in a go-routine and
// updates count by one every time a packet comes in
func (c *rCounter) Read() {
	for range c.input {
		c.Increment()
	}
}

func TestRepeat(test *testing.T) {
	assert := asrt.New(test)

	dur := time.Duration(50) * time.Millisecond
	short := time.Duration(20) * time.Millisecond
	// delay waits for cnt durations, an a little extra
	delay := func(cnt int) time.Duration {
		return time.Duration(cnt)*dur + time.Millisecond
	}
	t := NewRepeatTimer("bar", dur)

	// start at 0
	c := &rCounter{input: t.Ch}
	go c.Read()
	assert.Equal(0, c.Count())

	// wait for 4 periods
	time.Sleep(delay(4))
	assert.Equal(4, c.Count())

	// keep reseting leads to no firing
	for i := 0; i < 20; i++ {
		time.Sleep(short)
		t.Reset()
	}
	assert.Equal(4, c.Count())

	// after this, it still works normal
	time.Sleep(delay(2))
	assert.Equal(6, c.Count())

	// after a stop, nothing more is sent
	stopped := t.Stop()
	assert.True(stopped)
	time.Sleep(delay(7))
	assert.Equal(6, c.Count())

	// close channel to stop counter
	close(t.Ch)
}
