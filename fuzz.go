package p2p

import (
	"math/rand"
	"net"
	"sync"
	"time"

	cfg "github.com/tendermint/go-config"
)

//--------------------------------------------------------
// delay reads/writes
// randomly drop reads/writes
// randomly drop connections

const (
	FuzzModeDrop  = "drop"
	FuzzModeDelay = "delay"
)

func FuzzConn(config cfg.Config, conn net.Conn) net.Conn {
	return &FuzzedConnection{
		conn:   conn,
		start:  time.After(time.Second * 10), // so we have time to do peer handshakes and get set up
		params: config,
	}
}

type FuzzedConnection struct {
	conn net.Conn

	mtx   sync.Mutex
	fuzz  bool // we don't start fuzzing right away
	start <-chan time.Time

	// fuzz params
	params cfg.Config
}

func (fc *FuzzedConnection) randomDuration() time.Duration {
	return time.Millisecond * time.Duration(rand.Int()%fc.MaxDelayMilliseconds())
}

func (fc *FuzzedConnection) Active() bool {
	return fc.params.GetBool(configFuzzActive)
}

func (fc *FuzzedConnection) Mode() string {
	return fc.params.GetString(configFuzzMode)
}

func (fc *FuzzedConnection) ProbDropRW() float64 {
	return fc.params.GetFloat64(configFuzzProbDropRW)
}

func (fc *FuzzedConnection) ProbDropConn() float64 {
	return fc.params.GetFloat64(configFuzzProbDropConn)
}

func (fc *FuzzedConnection) ProbSleep() float64 {
	return fc.params.GetFloat64(configFuzzProbSleep)
}

func (fc *FuzzedConnection) MaxDelayMilliseconds() int {
	return fc.params.GetInt(configFuzzMaxDelayMilliseconds)
}

// implements the fuzz (delay, kill conn)
// and returns whether or not the read/write should be ignored
func (fc *FuzzedConnection) Fuzz() bool {
	if !fc.shouldFuzz() {
		return false
	}

	switch fc.Mode() {
	case FuzzModeDrop:
		// randomly drop the r/w, drop the conn, or sleep
		r := rand.Float64()
		if r <= fc.ProbDropRW() {
			return true
		} else if r < fc.ProbDropRW()+fc.ProbDropConn() {
			// XXX: can't this fail because machine precision?
			// XXX: do we need an error?
			fc.Close()
			return true
		} else if r < fc.ProbDropRW()+fc.ProbDropConn()+fc.ProbSleep() {
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
	if !fc.Active() {
		return false
	}

	fc.mtx.Lock()
	defer fc.mtx.Unlock()
	if fc.fuzz {
		return true
	}

	select {
	case <-fc.start:
		fc.fuzz = true
	default:
	}
	return false
}

func (fc *FuzzedConnection) Read(data []byte) (n int, err error) {
	if fc.Fuzz() {
		return 0, nil
	}
	return fc.conn.Read(data)
}

func (fc *FuzzedConnection) Write(data []byte) (n int, err error) {
	if fc.Fuzz() {
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
