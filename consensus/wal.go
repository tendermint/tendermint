package consensus

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"path/filepath"
	"time"

	"github.com/pkg/errors"

	amino "github.com/tendermint/go-amino"
	auto "github.com/tendermint/tendermint/libs/autofile"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
)

const (
	// must be greater than types.BlockPartSizeBytes + a few bytes
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

func RegisterWALMessages(cdc *amino.Codec) {
	cdc.RegisterInterface((*WALMessage)(nil), nil)
	cdc.RegisterConcrete(types.EventDataRoundState{}, "tendermint/wal/EventDataRoundState", nil)
	cdc.RegisterConcrete(msgInfo{}, "tendermint/wal/MsgInfo", nil)
	cdc.RegisterConcrete(timeoutInfo{}, "tendermint/wal/TimeoutInfo", nil)
	cdc.RegisterConcrete(EndHeightMessage{}, "tendermint/wal/EndHeightMessage", nil)
}

//--------------------------------------------------------
// Simple write-ahead logger

// WAL is an interface for any write-ahead logger.
type WAL interface {
	Write(WALMessage)
	WriteSync(WALMessage)
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

	enc *WALEncoder
}

func NewWAL(walFile string, groupOptions ...func(*auto.Group)) (*baseWAL, error) {
	err := cmn.EnsureDir(filepath.Dir(walFile), 0700)
	if err != nil {
		return nil, errors.Wrap(err, "failed to ensure WAL directory is in place")
	}

	group, err := auto.OpenGroup(walFile, groupOptions...)
	if err != nil {
		return nil, err
	}
	wal := &baseWAL{
		group: group,
		enc:   NewWALEncoder(group),
	}
	wal.BaseService = *cmn.NewBaseService(nil, "baseWAL", wal)
	return wal, nil
}

func (wal *baseWAL) Group() *auto.Group {
	return wal.group
}

func (wal *baseWAL) SetLogger(l log.Logger) {
	wal.BaseService.Logger = l
	wal.group.SetLogger(l)
}

func (wal *baseWAL) OnStart() error {
	size, err := wal.group.Head.Size()
	if err != nil {
		return err
	} else if size == 0 {
		wal.WriteSync(EndHeightMessage{0})
	}
	err = wal.group.Start()
	return err
}

// Stop the underlying autofile group.
// Use Wait() to ensure it's finished shutting down
// before cleaning up files.
func (wal *baseWAL) OnStop() {
	wal.group.Flush()
	wal.group.Stop()
	wal.group.Close()
}

// Wait for the underlying autofile group to finish shutting down
// so it's safe to cleanup files.
func (wal *baseWAL) Wait() {
	wal.group.Wait()
}

// Write is called in newStep and for each receive on the
// peerMsgQueue and the timeoutTicker.
// NOTE: does not call fsync()
func (wal *baseWAL) Write(msg WALMessage) {
	if wal == nil {
		return
	}

	// Write the wal message
	if err := wal.enc.Encode(&TimedWALMessage{tmtime.Now(), msg}); err != nil {
		panic(fmt.Sprintf("Error writing msg to consensus wal: %v \n\nMessage: %v", err, msg))
	}
}

// WriteSync is called when we receive a msg from ourselves
// so that we write to disk before sending signed messages.
// NOTE: calls fsync()
func (wal *baseWAL) WriteSync(msg WALMessage) {
	if wal == nil {
		return
	}

	wal.Write(msg)
	if err := wal.group.Flush(); err != nil {
		panic(fmt.Sprintf("Error flushing consensus wal buf to file. Error: %v \n", err))
	}
}

// WALSearchOptions are optional arguments to SearchForEndHeight.
type WALSearchOptions struct {
	// IgnoreDataCorruptionErrors set to true will result in skipping data corruption errors.
	IgnoreDataCorruptionErrors bool
}

// SearchForEndHeight searches for the EndHeightMessage with the given height
// and returns an auto.GroupReader, whenever it was found or not and an error.
// Group reader will be nil if found equals false.
//
// CONTRACT: caller must close group reader.
func (wal *baseWAL) SearchForEndHeight(height int64, options *WALSearchOptions) (gr *auto.GroupReader, found bool, err error) {
	var msg *TimedWALMessage
	lastHeightFound := int64(-1)

	// NOTE: starting from the last file in the group because we're usually
	// searching for the last height. See replay.go
	min, max := wal.group.MinIndex(), wal.group.MaxIndex()
	wal.Logger.Info("Searching for height", "height", height, "min", min, "max", max)
	for index := max; index >= min; index-- {
		gr, err = wal.group.NewReader(index)
		if err != nil {
			return nil, false, err
		}

		dec := NewWALDecoder(gr)
		for {
			msg, err = dec.Decode()
			if err == io.EOF {
				// OPTIMISATION: no need to look for height in older files if we've seen h < height
				if lastHeightFound > 0 && lastHeightFound < height {
					gr.Close()
					return nil, false, nil
				}
				// check next file
				break
			}
			if options.IgnoreDataCorruptionErrors && IsDataCorruptionError(err) {
				wal.Logger.Error("Corrupted entry. Skipping...", "err", err)
				// do nothing
				continue
			} else if err != nil {
				gr.Close()
				return nil, false, err
			}

			if m, ok := msg.Msg.(EndHeightMessage); ok {
				lastHeightFound = m.Height
				if m.Height == height { // found
					wal.Logger.Info("Found", "height", height, "index", index)
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
// Format: 4 bytes CRC sum + 4 bytes length + arbitrary-length value (go-amino encoded)
type WALEncoder struct {
	wr io.Writer
}

// NewWALEncoder returns a new encoder that writes to wr.
func NewWALEncoder(wr io.Writer) *WALEncoder {
	return &WALEncoder{wr}
}

// Encode writes the custom encoding of v to the stream. It returns an error if
// the amino-encoded size of v is greater than 1MB. Any error encountered
// during the write is also returned.
func (enc *WALEncoder) Encode(v *TimedWALMessage) error {
	data := cdc.MustMarshalBinaryBare(v)

	crc := crc32.Checksum(data, crc32c)
	length := uint32(len(data))
	if length > maxMsgSizeBytes {
		return fmt.Errorf("Msg is too big: %d bytes, max: %d bytes", length, maxMsgSizeBytes)
	}
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
		return nil, DataCorruptionError{fmt.Errorf("failed to read checksum: %v", err)}
	}
	crc := binary.BigEndian.Uint32(b)

	b = make([]byte, 4)
	_, err = dec.rd.Read(b)
	if err != nil {
		return nil, DataCorruptionError{fmt.Errorf("failed to read length: %v", err)}
	}
	length := binary.BigEndian.Uint32(b)

	if length > maxMsgSizeBytes {
		return nil, DataCorruptionError{fmt.Errorf("length %d exceeded maximum possible value of %d bytes", length, maxMsgSizeBytes)}
	}

	data := make([]byte, length)
	n, err := dec.rd.Read(data)
	if err != nil {
		return nil, DataCorruptionError{fmt.Errorf("failed to read data: %v (read: %d, wanted: %d)", err, n, length)}
	}

	// check checksum before decoding data
	actualCRC := crc32.Checksum(data, crc32c)
	if actualCRC != crc {
		return nil, DataCorruptionError{fmt.Errorf("checksums do not match: read: %v, actual: %v", crc, actualCRC)}
	}

	var res = new(TimedWALMessage) // nolint: gosimple
	err = cdc.UnmarshalBinaryBare(data, res)
	if err != nil {
		return nil, DataCorruptionError{fmt.Errorf("failed to decode data: %v", err)}
	}

	return res, err
}

type nilWAL struct{}

func (nilWAL) Write(m WALMessage)     {}
func (nilWAL) WriteSync(m WALMessage) {}
func (nilWAL) Group() *auto.Group     { return nil }
func (nilWAL) SearchForEndHeight(height int64, options *WALSearchOptions) (gr *auto.GroupReader, found bool, err error) {
	return nil, false, nil
}
func (nilWAL) Start() error { return nil }
func (nilWAL) Stop() error  { return nil }
func (nilWAL) Wait()        {}
