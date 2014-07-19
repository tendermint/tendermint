package blocks

import (
	db_ "github.com/tendermint/tendermint/db"
	"github.com/tendermint/tendermint/p2p"
)

const (
	blocksCh = "block"

	msgTypeUnknown = Byte(0x00)
	msgTypeState   = Byte(0x01)
	msgTypeRequest = Byte(0x02)
	msgTypeData    = Byte(0x03)

	DataTypeFullBlock  = byte(0x00)
	DataTypeValidation = byte(0x01)
	DataTypeTxs        = byte(0x02)
	// dataTypeCheckpoint = byte(0x04)

	dbKeyState = "state"
)

/*
 */
type BlockManager struct {
	db         *db_.LevelDB
	sw         *p2p.Switch
	swEvents   chan interface{}
	state      blockManagerState
	peerStates map[string]*blockManagerState
	quit       chan struct{}
	started    uint32
	stopped    uint32
}

func NewBlockManager(sw *p2p.Switch, db *db_.LevelDB) *BlockManager {
	swEvents := make(chan interface{})
	sw.AddEventListener("BlockManager.swEvents", swEvents)
	bm := &BlockManager{
		db:         db,
		sw:         sw,
		swEvents:   swEvents,
		peerStates: make(map[string]*blockManagerState),
		quit:       make(chan struct{}),
	}
	bm.loadState()
	return bm
}

func (bm *BlockManager) Start() {
	if atomic.CompareAndSwapUint32(&bm.started, 0, 1) {
		log.Info("Starting BlockManager")
		go bm.switchEventsHandler()
	}
}

func (bm *BlockManager) Stop() {
	if atomic.CompareAndSwapUint32(&bm.stopped, 0, 1) {
		log.Info("Stopping BlockManager")
		close(bm.quit)
		close(bm.swEvents)
	}
}

func (bm *BlockManager) StoreData(dataType byte, dataObj interface{}) {
	// Validate data if possible.
	// If we have new data that extends our contiguous range, then announce it.
}

func (bm *BlockManager) LoadData(dataType byte) interface{} {
	// NOTE: who's calling?
}

func (bm *BlockManager) loadState() {
	// Load the state
	stateBytes := bm.db.Get(dbKeyState)
	if stateBytes == nil {
		log.Info("New BlockManager with no state")
	} else {
		err := json.Unmarshal(stateBytes, &bm.state)
		if err != nil {
			panic("Could not unmarshal state bytes: %X", stateBytes)
		}
	}
}

func (bm *BlockManager) saveState() {
	stateBytes, err := json.Marshal(&bm.state)
	if err != nil {
		panic("Could not marshal state bytes")
	}
	bm.db.Set(dbKeyState, stateBytes)
}

func (bm *BlockManager) switchEventsHandler() {
	for {
		swEvent, ok := <-bm.swEvents
		if !ok {
			break
		}
		switch swEvent.(type) {
		case p2p.SwitchEventNewPeer:
			event := swEvent.(p2p.SwitchEventNewPeer)
			// Create entry in .peerStates
			// Share our state with event.Peer
			event.Peer
		case p2p.SwitchEventDonePeer:
			// Remove entry from .peerStates
		}
	}
}

//-----------------------------------------------------------------------------

/* This is just to persist the block manager state in the db. */
type blockManagerState struct {
	LastHeaderHeight       uint64 // Last contiguous header height
	OtherHeaderHeights     []uint64
	LastValidationHeight   uint64 // Last contiguous validation height
	OtherValidationHeights []uint64
	LastTxsHeight          uint64 // Last contiguous txs height
	OtherTxsHeights        []uint64
}

//-----------------------------------------------------------------------------

/*
Each part of a block are stored separately in the db.
*/

func headerKey(height int) {
	return fmt.Sprintf("B%v", height)
}

func validationKey(height int) {
	return fmt.Sprintf("V%v", height)
}

func txsKey(height int) {
	return fmt.Sprintf("T%v", height)
}

func checkpointKey(height int) {
	return fmt.Sprintf("C%v", height)
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
