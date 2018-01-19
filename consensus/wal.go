package consensus

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"path/filepath"
	"time"

	"github.com/pkg/errors"

	wire "github.com/tendermint/go-wire"
	"github.com/tendermint/tendermint/types"
	auto "github.com/tendermint/tmlibs/autofile"
	cmn "github.com/tendermint/tmlibs/common"
)

const (
	// must be greater than params.BlockGossip.BlockPartSizeBytes + a few bytes
	maxMsgSizeBytes = 1024 * 1024 // 1MB
)

//--------------------------------------------------------
// types and functions for savings consensus messages

type TimedWALMessage struct {
	Time time.Time  `json:"time"` // for debugging purposes
	Msg  WALMessage `json:"msg"`
}

// EndHeightMessage marks the end of the given height inside WAL.
// @internal used by scripts/wal2json util.
type EndHeightMessage struct {
	Height int64 `json:"height"`
}

type WALMessage interface{}

var _ = wire.RegisterInterface(
	struct{ WALMessage }{},
	wire.ConcreteType{types.EventDataRoundState{}, 0x01},
	wire.ConcreteType{msgInfo{}, 0x02},
	wire.ConcreteType{timeoutInfo{}, 0x03},
	wire.ConcreteType{EndHeightMessage{}, 0x04},
)

//--------------------------------------------------------
// Simple write-ahead logger

// WAL is an interface for any write-ahead logger.
type WAL interface {
	Save(WALMessage)
	Group() *auto.Group
	SearchForEndHeight(height int64, options *WALSearchOptions) (gr *auto.GroupReader, found bool, err error)

	Start() error
	Stop() error
	Wait()
}

// Write ahead logger writes msgs to disk before they are processed.
// Can be used for crash-recovery and deterministic replay
// TODO: currently the wal is overwritten during replay catchup
//   give it a mode so it's either reading or appending - must read to end to start appending again
type baseWAL struct {
	cmn.BaseService

	group *auto.Group
	light bool // ignore block parts

	enc *WALEncoder
}

func NewWAL(walFile string, light bool) (*baseWAL, error) {
	err := cmn.EnsureDir(filepath.Dir(walFile), 0700)
	if err != nil {
		return nil, errors.Wrap(err, "failed to ensure WAL directory is in place")
	}

	group, err := auto.OpenGroup(walFile)
	if err != nil {
		return nil, err
	}
	wal := &baseWAL{
		group: group,
		light: light,
		enc:   NewWALEncoder(group),
	}
	wal.BaseService = *cmn.NewBaseService(nil, "baseWAL", wal)
	return wal, nil
}

func (wal *baseWAL) Group() *auto.Group {
	return wal.group
}

func (wal *baseWAL) OnStart() error {
	size, err := wal.group.Head.Size()
	if err != nil {
		return err
	} else if size == 0 {
		wal.Save(EndHeightMessage{0})
	}
	err = wal.group.Start()
	return err
}

func (wal *baseWAL) OnStop() {
	wal.BaseService.OnStop()
	wal.group.Stop()
}

// called in newStep and for each pass in receiveRoutine
func (wal *baseWAL) Save(msg WALMessage) {
	if wal == nil {
		return
	}

	if wal.light {
		// in light mode we only write new steps, timeouts, and our own votes (no proposals, block parts)
		if mi, ok := msg.(msgInfo); ok {
			if mi.PeerID != "" {
				return
			}
		}
	}

	// Write the wal message
	if err := wal.enc.Encode(&TimedWALMessage{time.Now(), msg}); err != nil {
		cmn.PanicQ(cmn.Fmt("Error writing msg to consensus wal: %v \n\nMessage: %v", err, msg))
	}

	// TODO: only flush when necessary
	if err := wal.group.Flush(); err != nil {
		cmn.PanicQ(cmn.Fmt("Error flushing consensus wal buf to file. Error: %v \n", err))
	}
}

// WALSearchOptions are optional arguments to SearchForEndHeight.
type WALSearchOptions struct {
	// IgnoreDataCorruptionErrors set to true will result in skipping data corruption errors.
	IgnoreDataCorruptionErrors bool
}

// SearchForEndHeight searches for the EndHeightMessage with the height and
// returns an auto.GroupReader, whenever it was found or not and an error.
// Group reader will be nil if found equals false.
//
// CONTRACT: caller must close group reader.
func (wal *baseWAL) SearchForEndHeight(height int64, options *WALSearchOptions) (gr *auto.GroupReader, found bool, err error) {
	var msg *TimedWALMessage

	// NOTE: starting from the last file in the group because we're usually
	// searching for the last height. See replay.go
	min, max := wal.group.MinIndex(), wal.group.MaxIndex()
	wal.Logger.Debug("Searching for height", "height", height, "min", min, "max", max)
	for index := max; index >= min; index-- {
		gr, err = wal.group.NewReader(index)
		if err != nil {
			return nil, false, err
		}

		dec := NewWALDecoder(gr)
		for {
			msg, err = dec.Decode()
			if err == io.EOF {
				// check next file
				break
			}
			if options.IgnoreDataCorruptionErrors && IsDataCorruptionError(err) {
				// do nothing
			} else if err != nil {
				gr.Close()
				return nil, false, err
			}

			if m, ok := msg.Msg.(EndHeightMessage); ok {
				if m.Height == height { // found
					wal.Logger.Debug("Found", "height", height, "index", index)
					return gr, true, nil
				}
			}
		}
		gr.Close()
	}

	return nil, false, nil
}

///////////////////////////////////////////////////////////////////////////////

// A WALEncoder writes custom-encoded WAL messages to an output stream.
//
// Format: 4 bytes CRC sum + 4 bytes length + arbitrary-length value (go-wire encoded)
type WALEncoder struct {
	wr io.Writer
}

// NewWALEncoder returns a new encoder that writes to wr.
func NewWALEncoder(wr io.Writer) *WALEncoder {
	return &WALEncoder{wr}
}

// Encode writes the custom encoding of v to the stream.
func (enc *WALEncoder) Encode(v *TimedWALMessage) error {
	data := wire.BinaryBytes(v)

	crc := crc32.Checksum(data, crc32c)
	length := uint32(len(data))
	totalLength := 8 + int(length)

	msg := make([]byte, totalLength)
	binary.BigEndian.PutUint32(msg[0:4], crc)
	binary.BigEndian.PutUint32(msg[4:8], length)
	copy(msg[8:], data)

	_, err := enc.wr.Write(msg)

	return err
}

///////////////////////////////////////////////////////////////////////////////

// IsDataCorruptionError returns true if data has been corrupted inside WAL.
func IsDataCorruptionError(err error) bool {
	_, ok := err.(DataCorruptionError)
	return ok
}

// DataCorruptionError is an error that occures if data on disk was corrupted.
type DataCorruptionError struct {
	cause error
}

func (e DataCorruptionError) Error() string {
	return fmt.Sprintf("DataCorruptionError[%v]", e.cause)
}

func (e DataCorruptionError) Cause() error {
	return e.cause
}

// A WALDecoder reads and decodes custom-encoded WAL messages from an input
// stream. See WALEncoder for the format used.
//
// It will also compare the checksums and make sure data size is equal to the
// length from the header. If that is not the case, error will be returned.
type WALDecoder struct {
	rd io.Reader
}

// NewWALDecoder returns a new decoder that reads from rd.
func NewWALDecoder(rd io.Reader) *WALDecoder {
	return &WALDecoder{rd}
}

// Decode reads the next custom-encoded value from its reader and returns it.
func (dec *WALDecoder) Decode() (*TimedWALMessage, error) {
	b := make([]byte, 4)

	_, err := dec.rd.Read(b)
	if err == io.EOF {
		return nil, err
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read checksum: %v", err)
	}
	crc := binary.BigEndian.Uint32(b)

	b = make([]byte, 4)
	_, err = dec.rd.Read(b)
	if err == io.EOF {
		return nil, err
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read length: %v", err)
	}
	length := binary.BigEndian.Uint32(b)

	if length > maxMsgSizeBytes {
		return nil, DataCorruptionError{fmt.Errorf("length %d exceeded maximum possible value of %d bytes", length, maxMsgSizeBytes)}
	}

	data := make([]byte, length)
	_, err = dec.rd.Read(data)
	if err == io.EOF {
		return nil, err
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read data: %v", err)
	}

	// check checksum before decoding data
	actualCRC := crc32.Checksum(data, crc32c)
	if actualCRC != crc {
		return nil, DataCorruptionError{fmt.Errorf("checksums do not match: (read: %v, actual: %v)", crc, actualCRC)}
	}

	var nn int
	var res *TimedWALMessage // nolint: gosimple
	res = wire.ReadBinary(&TimedWALMessage{}, bytes.NewBuffer(data), int(length), &nn, &err).(*TimedWALMessage)
	if err != nil {
		return nil, DataCorruptionError{fmt.Errorf("failed to decode data: %v", err)}
	}

	return res, err
}

type nilWAL struct{}

func (nilWAL) Save(m WALMessage)  {}
func (nilWAL) Group() *auto.Group { return nil }
func (nilWAL) SearchForEndHeight(height int64, options *WALSearchOptions) (gr *auto.GroupReader, found bool, err error) {
	return nil, false, nil
}
func (nilWAL) Start() error { return nil }
func (nilWAL) Stop() error  { return nil }
func (nilWAL) Wait()        {}
