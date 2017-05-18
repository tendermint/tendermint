package consensus

import (
	"time"

	wire "github.com/tendermint/go-wire"
	"github.com/tendermint/tendermint/types"
	auto "github.com/tendermint/tmlibs/autofile"
	. "github.com/tendermint/tmlibs/common"
)

//--------------------------------------------------------
// types and functions for savings consensus messages

type TimedWALMessage struct {
	Time time.Time  `json:"time"`
	Msg  WALMessage `json:"msg"`
}

type WALMessage interface{}

var _ = wire.RegisterInterface(
	struct{ WALMessage }{},
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
	BaseService

	group *auto.Group
	light bool // ignore block parts
}

func NewWAL(walFile string, light bool) (*WAL, error) {
	group, err := auto.OpenGroup(walFile)
	if err != nil {
		return nil, err
	}
	wal := &WAL{
		group: group,
		light: light,
	}
	wal.BaseService = *NewBaseService(nil, "WAL", wal)
	return wal, nil
}

func (wal *WAL) OnStart() error {
	size, err := wal.group.Head.Size()
	if err != nil {
		return err
	} else if size == 0 {
		wal.writeEndHeight(0)
	}
	_, err = wal.group.Start()
	return err
}

func (wal *WAL) OnStop() {
	wal.BaseService.OnStop()
	wal.group.Stop()
}

// called in newStep and for each pass in receiveRoutine
func (wal *WAL) Save(wmsg WALMessage) {
	if wal == nil {
		return
	}
	if wal.light {
		// in light mode we only write new steps, timeouts, and our own votes (no proposals, block parts)
		if mi, ok := wmsg.(msgInfo); ok {
			if mi.PeerKey != "" {
				return
			}
		}
	}
	// Write the wal message
	var wmsgBytes = wire.JSONBytes(TimedWALMessage{time.Now(), wmsg})
	err := wal.group.WriteLine(string(wmsgBytes))
	if err != nil {
		PanicQ(Fmt("Error writing msg to consensus wal. Error: %v \n\nMessage: %v", err, wmsg))
	}
	// TODO: only flush when necessary
	if err := wal.group.Flush(); err != nil {
		PanicQ(Fmt("Error flushing consensus wal buf to file. Error: %v \n", err))
	}
}

func (wal *WAL) writeEndHeight(height int) {
	wal.group.WriteLine(Fmt("#ENDHEIGHT: %v", height))

	// TODO: only flush when necessary
	if err := wal.group.Flush(); err != nil {
		PanicQ(Fmt("Error flushing consensus wal buf to file. Error: %v \n", err))
	}
}
