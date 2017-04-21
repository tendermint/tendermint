package p2p

import (
	"math/rand"
	"net"
	"sync"
	"time"
)

const (
	// FuzzModeDrop is a mode in which we randomly drop reads/writes, connections or sleep
	FuzzModeDrop = iota
	// FuzzModeDelay is a mode in which we randomly sleep
	FuzzModeDelay
)

// FuzzedConnection wraps any net.Conn and depending on the mode either delays
// reads/writes or randomly drops reads/writes/connections.
type FuzzedConnection struct {
	conn net.Conn

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
func FuzzConn(conn net.Conn) net.Conn {
	return FuzzConnFromConfig(conn, DefaultFuzzConnConfig())
}

// FuzzConnFromConfig creates a new FuzzedConnection from a config. Fuzzing
// starts immediately.
func FuzzConnFromConfig(conn net.Conn, config *FuzzConnConfig) net.Conn {
	return &FuzzedConnection{
		conn:   conn,
		start:  make(<-chan time.Time),
		active: true,
		config: config,
	}
}

// FuzzConnAfter creates a new FuzzedConnection. Fuzzing starts when the
// duration elapses.
func FuzzConnAfter(conn net.Conn, d time.Duration) net.Conn {
	return FuzzConnAfterFromConfig(conn, d, DefaultFuzzConnConfig())
}

// FuzzConnAfterFromConfig creates a new FuzzedConnection from a config.
// Fuzzing starts when the duration elapses.
func FuzzConnAfterFromConfig(conn net.Conn, d time.Duration, config *FuzzConnConfig) net.Conn {
	return &FuzzedConnection{
		conn:   conn,
		start:  time.After(d),
		active: false,
		config: config,
	}
}

// Config returns the connection's config.
func (fc *FuzzedConnection) Config() *FuzzConnConfig {
	return fc.config
}

// Read implements net.Conn.
func (fc *FuzzedConnection) Read(data []byte) (n int, err error) {
	if fc.fuzz() {
		return 0, nil
	}
	return fc.conn.Read(data)
}

// Write implements net.Conn.
func (fc *FuzzedConnection) Write(data []byte) (n int, err error) {
	if fc.fuzz() {
		return 0, nil
	}
	return fc.conn.Write(data)
}

// Close implements net.Conn.
func (fc *FuzzedConnection) Close() error { return fc.conn.Close() }

// LocalAddr implements net.Conn.
func (fc *FuzzedConnection) LocalAddr() net.Addr { return fc.conn.LocalAddr() }

// RemoteAddr implements net.Conn.
func (fc *FuzzedConnection) RemoteAddr() net.Addr { return fc.conn.RemoteAddr() }

// SetDeadline implements net.Conn.
func (fc *FuzzedConnection) SetDeadline(t time.Time) error { return fc.conn.SetDeadline(t) }

// SetReadDeadline implements net.Conn.
func (fc *FuzzedConnection) SetReadDeadline(t time.Time) error {
	return fc.conn.SetReadDeadline(t)
}

// SetWriteDeadline implements net.Conn.
func (fc *FuzzedConnection) SetWriteDeadline(t time.Time) error {
	return fc.conn.SetWriteDeadline(t)
}

func (fc *FuzzedConnection) randomDuration() time.Duration {
	maxDelayMillis := int(fc.config.MaxDelay.Nanoseconds() / 1000)
	return time.Millisecond * time.Duration(rand.Int()%maxDelayMillis)
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
			fc.Close()
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
