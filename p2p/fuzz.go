package p2p

import (
	"math/rand"
	"sync"
	"time"

	inet "github.com/libp2p/go-libp2p-net"
)

const (
	// FuzzModeDrop is a mode in which we randomly drop reads/writes, connections or sleep
	FuzzModeDrop = iota
	// FuzzModeDelay is a mode in which we randomly sleep
	FuzzModeDelay
)

// FuzzedConnection wraps any inet.Stream and depending on the mode either delays
// reads/writes or randomly drops reads/writes/connections.
type FuzzedConnection struct {
	inet.Stream

	mtx    sync.Mutex
	start  <-chan time.Time
	active bool

	config *FuzzConnConfig
}

// FuzzConnConfig is a FuzzedConnection configuration.
type FuzzConnConfig struct {
	Mode         int
	MaxDelay     time.Duration
	ProbDropRW   float64
	ProbDropConn float64
	ProbSleep    float64
}

// DefaultFuzzConnConfig returns the default config.
func DefaultFuzzConnConfig() *FuzzConnConfig {
	return &FuzzConnConfig{
		Mode:         FuzzModeDrop,
		MaxDelay:     3 * time.Second,
		ProbDropRW:   0.2,
		ProbDropConn: 0.00,
		ProbSleep:    0.00,
	}
}

// FuzzConn creates a new FuzzedConnection. Fuzzing starts immediately.
func FuzzConn(conn inet.Stream) inet.Stream {
	return FuzzConnFromConfig(conn, DefaultFuzzConnConfig())
}

// FuzzConnFromConfig creates a new FuzzedConnection from a config. Fuzzing
// starts immediately.
func FuzzConnFromConfig(conn inet.Stream, config *FuzzConnConfig) inet.Stream {
	return &FuzzedConnection{
		Stream: conn,
		start:  make(<-chan time.Time),
		active: true,
		config: config,
	}
}

// FuzzConnAfter creates a new FuzzedConnection. Fuzzing starts when the
// duration elapses.
func FuzzConnAfter(conn inet.Stream, d time.Duration) inet.Stream {
	return FuzzConnAfterFromConfig(conn, d, DefaultFuzzConnConfig())
}

// FuzzConnAfterFromConfig creates a new FuzzedConnection from a config.
// Fuzzing starts when the duration elapses.
func FuzzConnAfterFromConfig(conn inet.Stream, d time.Duration, config *FuzzConnConfig) inet.Stream {
	return &FuzzedConnection{
		Stream: conn,
		start:  time.After(d),
		active: false,
		config: config,
	}
}

// Config returns the connection's config.
func (fc *FuzzedConnection) Config() *FuzzConnConfig {
	return fc.config
}

// Read implements inet.Stream.
func (fc *FuzzedConnection) Read(data []byte) (n int, err error) {
	if fc.fuzz() {
		return 0, nil
	}
	return fc.Stream.Read(data)
}

// Write implements inet.Stream.
func (fc *FuzzedConnection) Write(data []byte) (n int, err error) {
	if fc.fuzz() {
		return 0, nil
	}
	return fc.Stream.Write(data)
}

func (fc *FuzzedConnection) randomDuration() time.Duration {
	maxDelayMillis := int(fc.config.MaxDelay.Nanoseconds() / 1000)
	return time.Millisecond * time.Duration(rand.Int()%maxDelayMillis) // nolint: gas
}

// implements the fuzz (delay, kill conn)
// and returns whether or not the read/write should be ignored
func (fc *FuzzedConnection) fuzz() bool {
	if !fc.shouldFuzz() {
		return false
	}

	switch fc.config.Mode {
	case FuzzModeDrop:
		// randomly drop the r/w, drop the conn, or sleep
		r := rand.Float64()
		if r <= fc.config.ProbDropRW {
			return true
		} else if r < fc.config.ProbDropRW+fc.config.ProbDropConn {
			// XXX: can't this fail because machine precision?
			// XXX: do we need an error?
			fc.Close() // nolint: errcheck, gas
			return true
		} else if r < fc.config.ProbDropRW+fc.config.ProbDropConn+fc.config.ProbSleep {
			time.Sleep(fc.randomDuration())
		}
	case FuzzModeDelay:
		// sleep a bit
		time.Sleep(fc.randomDuration())
	}
	return false
}

func (fc *FuzzedConnection) shouldFuzz() bool {
	if fc.active {
		return true
	}

	fc.mtx.Lock()
	defer fc.mtx.Unlock()

	select {
	case <-fc.start:
		fc.active = true
		return true
	default:
		return false
	}
}
