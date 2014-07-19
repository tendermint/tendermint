package blocks

import (
	"github.com/tendermint/tendermint/p2p"
)

const (
	BlocksCh = "block"

	msgTypeUnknown = Byte(0x00)
	msgTypeState   = Byte(0x01)
	msgTypeRequest = Byte(0x02)
	msgTypeData    = Byte(0x03)

	dataTypeAll        = byte(0x00)
	dataTypeValidation = byte(0x01)
	dataTypeTxs        = byte(0x02)
	// dataTypeCheckpoint = byte(0x04)
)

/*
 */
type BlockManager struct {
	quit    chan struct{}
	started uint32
	stopped uint32
}

func NewBlockManager() *BlockManager {
	bm := &BlockManager{
		sw:   sw,
		quit: make(chan struct{}),
	}
	return bm
}

func (bm *BlockManager) Start() {
	if atomic.CompareAndSwapUint32(&bm.started, 0, 1) {
		log.Info("Starting BlockManager")
		go bm.XXX()
	}
}

func (bm *BlockManager) Stop() {
	if atomic.CompareAndSwapUint32(&bm.stopped, 0, 1) {
		log.Info("Stopping BlockManager")
		close(bm.quit)
	}
}

func (bm *BlockManager) XXX() {
}

//-----------------------------------------------------------------------------

/* Messages */

// TODO: check for unnecessary extra bytes at the end.
func decodeMessage(bz ByteSlice) (msg Message) {
	// log.Debug("decoding msg bytes: %X", bz)
	switch Byte(bz[0]) {
	case msgTypeState:
		return &StateMessage{}
	case msgTypeRequest:
		return readRequestMessage(bytes.NewReader(bz[1:]))
	case msgTypeData:
		return readDataMessage(bytes.NewReader(bz[1:]))
	default:
		return nil
	}
}

/*
A StateMessage declares what (contiguous) blocks & headers are known.

LastValidationHeight >= LastBlockHeight.
*/
type StateMessage struct {
	LastBlockHeight      UInt64
	LastValidationHeight UInt64
}

func readStateMessage(r io.Reader) *StateMessage {
	lastBlockHeight := ReadUInt64(r)
	lastValidationHeight := ReadUInt64(r)
	return &StateMessage{
		LastBlockHeight:      lastBlockHeight,
		LastValidationHeight: lastValidationHeight,
	}
}

func (m *StateMessage) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteTo(msgTypeState, w, n, err)
	n, err = WriteTo(m.LastBlockHeight, w, n, err)
	n, err = WriteTo(m.LastValidationHeight, w, n, err)
	return
}

func (m *StateMessage) String() string {
	return fmt.Sprintf("[State %v/%v]",
		m.LastBlockHeight, m.LastValidationHeight)
}

/*
A RequestMessage requests a block and/or header at a given height.
*/
type RequestMessage struct {
	Type   Byte
	Height UInt64
}

func readRequestMessage(r io.Reader) *RequestMessage {
	requestType := ReadByte(r)
	height := ReadUInt64(r)
	return &RequestMessage{
		Type:   requestType,
		Height: height,
	}
}

func (m *RequestMessage) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteTo(msgTypeRequest, w, n, err)
	n, err = WriteTo(m.Type, w, n, err)
	n, err = WriteTo(m.Height, w, n, err)
	return
}

func (m *RequestMessage) String() string {
	return fmt.Sprintf("[Request %X@%v]", m.Type, m.Height)
}

/*
A DataMessage contains block data, maybe requested.
The data can be a Validation, Txs, or whole Block object.
*/
type DataMessage struct {
	Type   Byte
	Height UInt64
	Bytes  ByteSlice
}

func readDataMessage(r io.Reader) *DataMessage {
	dataType := ReadByte(r)
	height := ReadUInt64(r)
	bytes := ReadByteSlice(r)
	return &DataMessage{
		Type:   dataType,
		Height: height,
		Bytes:  bytes,
	}
}

func (m *DataMessage) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteTo(msgTypeData, w, n, err)
	n, err = WriteTo(m.Type, w, n, err)
	n, err = WriteTo(m.Height, w, n, err)
	n, err = WriteTo(m.Bytes, w, n, err)
	return
}

func (m *DataMessage) String() string {
	return fmt.Sprintf("[Data %X@%v]", m.Type, m.Height)
}
