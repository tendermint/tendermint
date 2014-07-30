package blocks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	db_ "github.com/tendermint/tendermint/db"
	"github.com/tendermint/tendermint/p2p"
)

var dbKeyState = []byte("state")

const (
	blocksInfoCh = byte(0x10) // For requests & cancellations
	blocksDataCh = byte(0x11) // For data

	msgTypeUnknown = Byte(0x00)
	msgTypeState   = Byte(0x01)
	msgTypeRequest = Byte(0x02)
	msgTypeData    = Byte(0x03)
)

/*
TODO: keep tabs on current active requests onPeerState.
TODO: keep a heap of dataRequests * their corresponding timeouts.
timeout dataRequests and update the peerState,
TODO: when a data item has bene received successfully, update the peerState.
ensure goroutine safety.
*/

//-----------------------------------------------------------------------------

const (
	dataTypeBlock = byte(0x00)
	// TODO: allow for more types, such as specific transactions
)

func computeDataKey(dataType byte, height uint64) string {
	switch dataType {
	case dataTypeBlock:
		return fmt.Sprintf("B%v", height)
	default:
		Panicf("Unknown datatype %X", dataType)
		return "" // should not happen
	}
}

//-----------------------------------------------------------------------------

type BlockManager struct {
	db         *db_.LevelDB
	sw         *p2p.Switch
	swEvents   chan interface{}
	state      blockManagerState
	dataStates map[string]*dataState // TODO: replace with CMap
	peerStates map[string]*peerState // TODO: replace with CMap
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
		dataStates: make(map[string]*dataState),
		peerStates: make(map[string]*peerState),
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

// NOTE: assumes that data is already validated.
// "request" is optional, it's the request response that supplied
// the data.
func (bm *BlockManager) StoreBlock(block *Block, origin *dataRequest) {
	dataKey := computeDataKey(dataTypeBlock, uint64(block.Header.Height))
	// Remove dataState entry, we'll no longer request this.
	_dataState := bm.dataStates[dataKey]
	removedRequests := _dataState.removeRequestsForDataType(dataTypeBlock)
	for _, request := range removedRequests {
		// Notify peer that the request has been canceled.
		if request.peer.Equals(origin.peer) {
			continue
		} else {
			// Send cancellation on blocksInfoCh channel
			msg := &requestMessage{
				dataType: Byte(dataTypeBlock),
				height:   block.Header.Height,
				canceled: Byte(0x01),
			}
			tm := p2p.TypedMessage{msgTypeRequest, msg}
			request.peer.TrySend(blocksInfoCh, tm.Bytes())
		}
		// Remove dataRequest from request.peer's peerState.
		peerState := bm.peerStates[request.peer.Key]
		peerState.remoteDataRequest(request)
	}
	// Update state
	newContiguousHeight := bm.state.addData(dataTypeBlock, uint64(block.Header.Height))
	// If we have new data that extends our contiguous range, then announce it.
	if newContiguousHeight {
		bm.sw.Broadcast(blocksInfoCh, bm.state.stateMessage())
	}
}

func (bm *BlockManager) LoadData(dataType byte, height uint64) interface{} {
	panic("not yet implemented")
}

func (bm *BlockManager) loadState() {
	// Load the state
	stateBytes := bm.db.Get(dbKeyState)
	if stateBytes == nil {
		log.Info("New BlockManager with no state")
	} else {
		err := json.Unmarshal(stateBytes, &bm.state)
		if err != nil {
			Panicf("Could not unmarshal state bytes: %X", stateBytes)
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

// Handle peer new/done events
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
			bm.peerStates[event.Peer.Key] = &peerState{}
			// Share our state with event.Peer
			msg := &stateMessage{
				lastBlockHeight: UInt64(bm.state.lastBlockHeight),
			}
			tm := p2p.TypedMessage{msgTypeRequest, msg}
			event.Peer.TrySend(blocksInfoCh, tm.Bytes())
		case p2p.SwitchEventDonePeer:
			event := swEvent.(p2p.SwitchEventDonePeer)
			// Remove entry from .peerStates
			delete(bm.peerStates, event.Peer.Key)
		default:
			log.Warning("Unhandled switch event type")
		}
	}
}

// Handle requests from the blocks channel
func (bm *BlockManager) requestsHandler() {
	for {
		inMsg, ok := bm.sw.Receive(blocksInfoCh)
		if !ok {
			// Client has stopped
			break
		}

		// decode message
		msg := decodeMessage(inMsg.Bytes)
		log.Info("requestHandler received %v", msg)

		switch msg.(type) {
		case *stateMessage:
			m := msg.(*stateMessage)
			peerState := bm.peerStates[inMsg.MConn.Peer.Key]
			if peerState == nil {
				continue // peer has since been disconnected.
			}
			peerState.applyStateMessage(m)
			// Consider requesting data.
			// 1. if has more validation and we want it
			// 2. if has more txs and we want it
			// if peerState.estimatedCredit() >= averageBlock

			// TODO: keep track of what we've requested from peer.
			// TODO: keep track of from which peers we've requested data.
			// TODO: be fair.
		case *requestMessage:
			// TODO: prevent abuse.
		case *dataMessage:
			// XXX move this to another channe
			// See if we want the data.
			// Validate data.
			// Add to db.
			// Update state & broadcast as necessary.
		default:
			// Ignore unknown message
			// bm.sw.StopPeerForError(inMsg.MConn.Peer, errInvalidMessage)
		}
	}

	// Cleanup
}

//-----------------------------------------------------------------------------

// blockManagerState keeps track of which block parts are stored locally.
// It's also persisted via JSON in the db.
type blockManagerState struct {
	mtx               sync.Mutex
	lastBlockHeight   uint64 // Last contiguous header height
	otherBlockHeights map[uint64]struct{}
}

func (bms blockManagerState) stateMessage() *stateMessage {
	bms.mtx.Lock()
	defer bms.mtx.Unlock()
	return &stateMessage{
		lastBlockHeight: UInt64(bms.lastBlockHeight),
	}
}

func (bms blockManagerState) addData(dataType byte, height uint64) bool {
	bms.mtx.Lock()
	defer bms.mtx.Unlock()
	if dataType != dataTypeBlock {
		Panicf("Unknown datatype %X", dataType)
	}
	if bms.lastBlockHeight == height-1 {
		bms.lastBlockHeight = height
		height++
		for _, ok := bms.otherBlockHeights[height]; ok; {
			delete(bms.otherBlockHeights, height)
			bms.lastBlockHeight = height
			height++
		}
		return true
	}
	return false
}

//-----------------------------------------------------------------------------

// dataRequest keeps track of each request for a given peice of data & peer.
type dataRequest struct {
	peer     *p2p.Peer
	dataType byte
	height   uint64
	time     time.Time // XXX keep track of timeouts.
}

//-----------------------------------------------------------------------------

// dataState keeps track of all requests for a given piece of data.
type dataState struct {
	mtx      sync.Mutex
	requests []*dataRequest
}

func (ds *dataState) removeRequestsForDataType(dataType byte) []*dataRequest {
	ds.mtx.Lock()
	defer ds.mtx.Lock()
	requests := []*dataRequest{}
	filtered := []*dataRequest{}
	for _, request := range ds.requests {
		if request.dataType == dataType {
			filtered = append(filtered, request)
		} else {
			requests = append(requests, request)
		}
	}
	ds.requests = requests
	return filtered
}

//-----------------------------------------------------------------------------

type peerState struct {
	mtx             sync.Mutex
	lastBlockHeight uint64 // Last contiguous header height
}

func (ps *peerState) applyStateMessage(msg *stateMessage) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	ps.lastBlockHeight = uint64(msg.lastBlockHeight)
}

func (ps *peerState) addDataRequest(request *dataRequest) {
	// TODO: keep track of dataRequests
}

func (ps *peerState) remoteDataRequest(request *dataRequest) {
	// TODO: keep track of dataRequests, and remove them here.
}

//-----------------------------------------------------------------------------

/* Messages */

// TODO: check for unnecessary extra bytes at the end.
func decodeMessage(bz ByteSlice) (msg interface{}) {
	// log.Debug("decoding msg bytes: %X", bz)
	switch Byte(bz[0]) {
	case msgTypeState:
		return &stateMessage{}
	case msgTypeRequest:
		return readRequestMessage(bytes.NewReader(bz[1:]))
	case msgTypeData:
		return readDataMessage(bytes.NewReader(bz[1:]))
	default:
		return nil
	}
}

/*
A stateMessage declares what (contiguous) blocks & headers are known.
*/
type stateMessage struct {
	lastBlockHeight UInt64 // Last contiguous block height
}

func readStateMessage(r io.Reader) *stateMessage {
	return &stateMessage{
		lastBlockHeight: ReadUInt64(r),
	}
}

func (m *stateMessage) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteTo(msgTypeState, w, n, err)
	n, err = WriteTo(m.lastBlockHeight, w, n, err)
	return
}

func (m *stateMessage) String() string {
	return fmt.Sprintf("[State B:%v]", m.lastBlockHeight)
}

/*
A requestMessage requests a block and/or header at a given height.
*/
type requestMessage struct {
	dataType Byte
	height   UInt64
	canceled Byte // 0x00 if request, 0x01 if cancellation
}

func readRequestMessage(r io.Reader) *requestMessage {
	return &requestMessage{
		dataType: ReadByte(r),
		height:   ReadUInt64(r),
		canceled: ReadByte(r),
	}
}

func (m *requestMessage) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteTo(msgTypeRequest, w, n, err)
	n, err = WriteTo(m.dataType, w, n, err)
	n, err = WriteTo(m.height, w, n, err)
	n, err = WriteTo(m.canceled, w, n, err)
	return
}

func (m *requestMessage) String() string {
	if m.canceled == Byte(0x01) {
		return fmt.Sprintf("[Cancellation %X@%v]", m.dataType, m.height)
	} else {
		return fmt.Sprintf("[Request %X@%v]", m.dataType, m.height)
	}
}

/*
A dataMessage contains block data, maybe requested.
The data can be a Validation, Txs, or whole Block object.
*/
type dataMessage struct {
	dataType Byte
	height   UInt64
	bytes    ByteSlice
}

func readDataMessage(r io.Reader) *dataMessage {
	dataType := ReadByte(r)
	height := ReadUInt64(r)
	bytes := ReadByteSlice(r)
	return &dataMessage{
		dataType: dataType,
		height:   height,
		bytes:    bytes,
	}
}

func (m *dataMessage) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteTo(msgTypeData, w, n, err)
	n, err = WriteTo(m.dataType, w, n, err)
	n, err = WriteTo(m.height, w, n, err)
	n, err = WriteTo(m.bytes, w, n, err)
	return
}

func (m *dataMessage) String() string {
	return fmt.Sprintf("[Data %X@%v]", m.dataType, m.height)
}
