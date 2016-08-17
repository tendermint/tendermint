package consensus

import (
	"bufio"
	"os"
	"time"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-wire"
	"github.com/tendermint/tendermint/types"
)

//--------------------------------------------------------
// types and functions for savings consensus messages

type ConsensusLogMessage struct {
	Time time.Time                    `json:"time"`
	Msg  ConsensusLogMessageInterface `json:"msg"`
}

type ConsensusLogMessageInterface interface{}

var _ = wire.RegisterInterface(
	struct{ ConsensusLogMessageInterface }{},
	wire.ConcreteType{types.EventDataRoundState{}, 0x01},
	wire.ConcreteType{msgInfo{}, 0x02},
	wire.ConcreteType{timeoutInfo{}, 0x03},
)

//--------------------------------------------------------
// Simple write-ahead logger

// Write ahead logger writes msgs to disk before they are processed.
// Can be used for crash-recovery and deterministic replay
// TODO: currently the wal is overwritten during replay catchup
//   give it a mode so it's either reading or appending - must read to end to start appending again
type WAL struct {
	fp     *os.File
	exists bool // if the file already existed (restarted process)

	done chan struct{}

	light bool // ignore block parts
}

func NewWAL(file string, light bool) (*WAL, error) {
	var walExists bool
	if _, err := os.Stat(file); err == nil {
		walExists = true
	}
	fp, err := os.OpenFile(file, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0600)
	if err != nil {
		return nil, err
	}
	return &WAL{
		fp:     fp,
		exists: walExists,
		done:   make(chan struct{}),
		light:  light,
	}, nil
}

// called in newStep and for each pass in receiveRoutine
func (wal *WAL) Save(clm ConsensusLogMessageInterface) {
	if wal != nil {
		wal.exists = true
		if wal.light {
			// in light mode we only write new steps, timeouts, and our own votes (no proposals, block parts)
			if mi, ok := clm.(msgInfo); ok {
				_ = mi
				if mi.PeerKey != "" {
					return
				}
			}
		}
		var n int
		var err error
		wire.WriteJSON(ConsensusLogMessage{time.Now(), clm}, wal.fp, &n, &err)
		wire.WriteTo([]byte("\n"), wal.fp, &n, &err) // one message per line
		if err != nil {
			PanicQ(Fmt("Error writing msg to consensus wal. Error: %v \n\nMessage: %v", err, clm))
		}
	}
}

// Must not be called concurrently with a write.
func (wal *WAL) Close() {
	if wal != nil {
		wal.fp.Close()
		wal.done <- struct{}{}
	}
}

func (wal *WAL) Wait() {
	if wal != nil {
		<-wal.done
	}
}

func (wal *WAL) SeekFromEnd(found func([]byte) bool) (nLines int, err error) {
	var current int64
	// start at the end
	current, err = wal.fp.Seek(0, 2)
	if err != nil {
		return
	}

	// backup until we find the the right line
	// current is how far we are from the beginning
	for {
		current -= 1
		if current < 0 {
			wal.fp.Seek(0, 0) // back to beginning
			return
		}
		// backup one and read a new byte
		if _, err = wal.fp.Seek(current, 0); err != nil {
			return
		}
		b := make([]byte, 1)
		if _, err = wal.fp.Read(b); err != nil {
			return
		}
		if b[0] == '\n' || len(b) == 0 {
			nLines += 1
			// read a full line
			reader := bufio.NewReader(wal.fp)
			lineBytes, _ := reader.ReadBytes('\n')
			if len(lineBytes) == 0 {
				continue
			}

			if found(lineBytes) {
				wal.fp.Seek(0, 1) // (?)
				wal.fp.Seek(current, 0)
				return
			}
		}
	}
}
