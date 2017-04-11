package p2p

import (
	"fmt"
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

type FuzzConnConfig struct {
	Mode         int
	MaxDelay     time.Duration
	ProbDropRW   float64
	ProbDropConn float64
	ProbSleep    float64
}

func defaultFuzzConnConfig() *FuzzConnConfig {
	return &FuzzConnConfig{
		Mode:         FuzzModeDrop,
		MaxDelay:     3 * time.Second,
		ProbDropRW:   0.2,
		ProbDropConn: 0.00,
		ProbSleep:    0.00,
	}
}

func FuzzConn(conn net.Conn) net.Conn {
	return FuzzConnFromConfig(conn, defaultFuzzConnConfig())
}

func FuzzConnFromConfig(conn net.Conn, config *FuzzConnConfig) net.Conn {
	return &FuzzedConnection{
		conn:         conn,
		start:        make(<-chan time.Time),
		active:       true,
		mode:         config.Mode,
		maxDelay:     config.MaxDelay,
		probDropRW:   config.ProbDropRW,
		probDropConn: config.ProbDropConn,
		probSleep:    config.ProbSleep,
	}
}

func FuzzConnAfter(conn net.Conn, d time.Duration) net.Conn {
	return FuzzConnAfterFromConfig(conn, d, defaultFuzzConnConfig())
}

func FuzzConnAfterFromConfig(conn net.Conn, d time.Duration, config *FuzzConnConfig) net.Conn {
	return &FuzzedConnection{
		conn:         conn,
		start:        time.After(d),
		active:       false,
		mode:         config.Mode,
		maxDelay:     config.MaxDelay,
		probDropRW:   config.ProbDropRW,
		probDropConn: config.ProbDropConn,
		probSleep:    config.ProbSleep,
	}
}

// FuzzedConnection wraps any net.Conn and depending on the mode either delays
// reads/writes or randomly drops reads/writes/connections.
type FuzzedConnection struct {
	conn net.Conn

	mtx    sync.Mutex
	start  <-chan time.Time
	active bool

	mode         int
	maxDelay     time.Duration
	probDropRW   float64
	probDropConn float64
	probSleep    float64
}

func (fc *FuzzedConnection) randomDuration() time.Duration {
	maxDelayMillis := int(fc.maxDelay.Nanoseconds() / 1000)
	return time.Millisecond * time.Duration(rand.Int()%maxDelayMillis)
}

func (fc *FuzzedConnection) SetMode(mode int) {
	switch mode {
	case FuzzModeDrop:
		fc.mode = FuzzModeDrop
	case FuzzModeDelay:
		fc.mode = FuzzModeDelay
	default:
		panic(fmt.Sprintf("Unknown mode %d", mode))
	}
}

func (fc *FuzzedConnection) SetProbDropRW(prob float64) {
	fc.probDropRW = prob
}

func (fc *FuzzedConnection) SetProbDropConn(prob float64) {
	fc.probDropConn = prob
}

func (fc *FuzzedConnection) SetProbSleep(prob float64) {
	fc.probSleep = prob
}

func (fc *FuzzedConnection) SetMaxDelay(d time.Duration) {
	fc.maxDelay = d
}

// implements the fuzz (delay, kill conn)
// and returns whether or not the read/write should be ignored
func (fc *FuzzedConnection) fuzz() bool {
	if !fc.shouldFuzz() {
		return false
	}

	switch fc.mode {
	case FuzzModeDrop:
		// randomly drop the r/w, drop the conn, or sleep
		r := rand.Float64()
		if r <= fc.probDropRW {
			return true
		} else if r < fc.probDropRW+fc.probDropConn {
			// XXX: can't this fail because machine precision?
			// XXX: do we need an error?
			fc.Close()
			return true
		} else if r < fc.probDropRW+fc.probDropConn+fc.probSleep {
			time.Sleep(fc.randomDuration())
		}
	case FuzzModeDelay:
		// sleep a bit
		time.Sleep(fc.randomDuration())
	}
	return false
}

// we don't fuzz until start chan fires
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

// Read implements net.Conn
func (fc *FuzzedConnection) Read(data []byte) (n int, err error) {
	if fc.fuzz() {
		return 0, nil
	}
	return fc.conn.Read(data)
}

// Write implements net.Conn
func (fc *FuzzedConnection) Write(data []byte) (n int, err error) {
	if fc.fuzz() {
		return 0, nil
	}
	return fc.conn.Write(data)
}

// Implements net.Conn
func (fc *FuzzedConnection) Close() error                  { return fc.conn.Close() }
func (fc *FuzzedConnection) LocalAddr() net.Addr           { return fc.conn.LocalAddr() }
func (fc *FuzzedConnection) RemoteAddr() net.Addr          { return fc.conn.RemoteAddr() }
func (fc *FuzzedConnection) SetDeadline(t time.Time) error { return fc.conn.SetDeadline(t) }
func (fc *FuzzedConnection) SetReadDeadline(t time.Time) error {
	return fc.conn.SetReadDeadline(t)
}
func (fc *FuzzedConnection) SetWriteDeadline(t time.Time) error {
	return fc.conn.SetWriteDeadline(t)
}
