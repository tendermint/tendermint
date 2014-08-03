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

	maxRequestsPerPeer   = 2 // Maximum number of outstanding requests from peer.
	maxRequestsPerData   = 2 // Maximum number of outstanding requests of some data.
	maxRequestAheadBlock = 5 // Maximum number of blocks to request ahead of current verified. Must be >= 1

	defaultRequestTimeoutS = 
	timeoutRepeatTimerMS = 1000 // Handle timed out requests periodically
)

/*
TODO: keep a heap of dataRequests * their corresponding timeouts.
timeout dataRequests and update the peerState,
TODO: need to keep track of progress, blocks are too large. or we need to chop into chunks.
TODO: need to validate blocks. :/
TODO: actually save the block.
*/

//-----------------------------------------------------------------------------

const (
	dataTypeBlock = byte(0x00)
	// TODO: allow for more types, such as specific transactions
)

type dataKey struct {
	dataType byte
	height   uint64
}

func newDataKey(dataType byte, height uint64) dataKey {
	return dataKey{dataType, height}
}

func readDataKey(r io.Reader) dataKey {
	return dataKey{
		dataType: ReadByte(r),
		height:   ReadUInt64(r),
	}
}

func (dk dataKey) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteTo(dk.dataType, w, n, err)
	n, err = WriteTo(dk.height, w, n, err)
	return
}

func (dk dataKey) String() string {
	switch dataType {
	case dataTypeBlock:
		return dataKeyfmt.Sprintf("B%v", height)
	default:
		Panicf("Unknown datatype %X", dataType)
		return "" // should not happen
	}
}

//-----------------------------------------------------------------------------

type BlockManager struct {
	db           *db_.LevelDB
	sw           *p2p.Switch
	swEvents     chan interface{}
	state        *blockManagerState
	timeoutTimer *RepeatTimer
	quit         chan struct{}
	started      uint32
	stopped      uint32
}

func NewBlockManager(sw *p2p.Switch, db *db_.LevelDB) *BlockManager {
	swEvents := make(chan interface{})
	sw.AddEventListener("BlockManager.swEvents", swEvents)
	bm := &BlockManager{
		db:           db,
		sw:           sw,
		swEvents:     swEvents,
		state:        newBlockManagerState(),
		timeoutTimer: NewRepeatTimer(timeoutRepeatTimerMS * time.Second),
		quit:         make(chan struct{}),
	}
	bm.loadState()
	return bm
}

func (bm *BlockManager) Start() {
	if atomic.CompareAndSwapUint32(&bm.started, 0, 1) {
		log.Info("Starting BlockManager")
		go bm.switchEventsHandler()
		go bm.blocksInfoHandler()
		go bm.blocksDataHandler()
		go bm.requestTimeoutHandler()
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
	dataKey := newDataKey(dataTypeBlock, uint64(block.Header.Height))

	// XXX actually save the block.

	canceled, newHeight := bm.state.didGetDataFromPeer(dataKey, origin.peer)

	// Notify peers that the request has been canceled.
	for _, request := range canceled {
		msg := &requestMessage{
			key:   dataKey,
			type_: requestTypeCanceled,
		}
		tm := p2p.TypedMessage{msgTypeRequest, msg}
		request.peer.TrySend(blocksInfoCh, tm.Bytes())
	}

	// If we have new data that extends our contiguous range, then announce it.
	if newHeight {
		bm.sw.Broadcast(blocksInfoCh, bm.state.makeStateMessage())
	}
}

func (bm *BlockManager) LoadBlock(height uint64) *Block {
	panic("not yet implemented")
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
			// Create peerState for event.Peer
			bm.state.createEntryForPeer(event.Peer)
			// Share our state with event.Peer
			msg := &stateMessage{
				lastBlockHeight: UInt64(bm.state.lastBlockHeight),
			}
			tm := p2p.TypedMessage{msgTypeRequest, msg}
			event.Peer.TrySend(blocksInfoCh, tm.Bytes())
		case p2p.SwitchEventDonePeer:
			event := swEvent.(p2p.SwitchEventDonePeer)
			// Delete peerState for event.Peer
			bm.state.deleteEntryForPeer(event.Peer)
		default:
			log.Warning("Unhandled switch event type")
		}
	}
}

// Handle requests/cancellations from the blocksInfo channel
func (bm *BlockManager) blocksInfoHandler() {
	for {
		inMsg, ok := bm.sw.Receive(blocksInfoCh)
		if !ok {
			break // Client has stopped
		}

		msg := decodeMessage(inMsg.Bytes)
		log.Info("blocksInfoHandler received %v", msg)

		switch msg.(type) {
		case *stateMessage:
			m := msg.(*stateMessage)
			peerState := bm.getPeerState(inMsg.MConn.Peer)
			if peerState == nil {
				continue // peer has since been disconnected.
			}
			newDataTypes := peerState.applyStateMessage(m)
			// Consider requesting data.
			// Does the peer claim to have something we want?
		FOR_LOOP:
			for _, newDataType := range newDataTypes {
				// Are we already requesting too much data from peer?
				if !peerState.canRequestMore() {
					break FOR_LOOP
				}
				for _, wantedKey := range bm.state.nextWantedKeysForType(newDataType) {
					if !peerState.hasData(wantedKey) {
						break FOR_LOOP
					}
					// Request wantedKey from peer.
					msg := &requestMessage{
						key:   dataKey,
						type_: requestTypeFetch,
					}
					tm := p2p.TypedMessage{msgTypeRequest, msg}
					sent := inMsg.MConn.Peer.TrySend(blocksInfoCh, tm.Bytes())
					if sent {
						// Log the request
						request := &dataRequest{
							peer: inMsg.MConn.Peer,
							key:  wantedKey,
							time: time.Now(),
							timeout: time.Now().Add(defaultRequestTimeout
						}
						bm.state.addDataRequest(request)
					}
				}
			}
		case *requestMessage:
			m := msg.(*requestMessage)
			switch m.type_ {
			case requestTypeFetch:
				// TODO: prevent abuse.
				if !inMsg.MConn.Peer.CanSend(blocksDataCh) {
					msg := &requestMessage{
						key:   dataKey,
						type_: requestTypeTryAgain,
					}
					tm := p2p.TypedMessage{msgTypeRequest, msg}
					sent := inMsg.MConn.Peer.TrySend(blocksInfoCh, tm.Bytes())
				} else {
					// If we don't have it, log and ignore.
					block := bm.LoadBlock(m.key.height)
					if block == nil {
						log.Warning("Peer %v asked for nonexistant block %v", inMsg.MConn.Peer, m.key)
					}
					// Send the data.
					msg := &dataMessage{
						key:   dataKey,
						bytes: BinaryBytes(block),
					}
					tm := p2p.TypedMessage{msgTypeData, msg}
					inMsg.MConn.Peer.TrySend(blocksDataCh, tm.Bytes())
				}
			case requestTypeCanceled:
				// TODO: handle
				// This requires modifying mconnection to keep track of item keys.
			case requestTypeTryAgain:
				// TODO: handle
			default:
				log.Warning("Invalid request: %v", m)
				// Ignore.
			}
		default:
			// should not happen
			Panicf("Unknown message %v", msg)
			// bm.sw.StopPeerForError(inMsg.MConn.Peer, errInvalidMessage)
		}
	}

	// Cleanup
}

// Handle receiving data from the blocksData channel
func (bm *BlockManager) blocksDataHandler() {
	for {
		inMsg, ok := bm.sw.Receive(blocksDataCh)
		if !ok {
			break // Client has stopped
		}

		msg := decodeMessage(inMsg.Bytes)
		log.Info("blocksDataHandler received %v", msg)

		switch msg.(type) {
		case *dataMessage:
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

// Handle timed out requests by requesting from others.
func (bm *BlockManager) requestTimeoutHandler() {
	for {
		_, ok := <-bm.timeoutTimer
		if !ok {
			break
		}
		// Iterate over requests by time and handle timed out requests.
	}
}

//-----------------------------------------------------------------------------

// blockManagerState keeps track of which block parts are stored locally.
// It's also persisted via JSON in the db.
type blockManagerState struct {
	mtx               sync.Mutex
	lastBlockHeight   uint64 // Last contiguous header height
	otherBlockHeights map[uint64]struct{}
	requestsByKey     map[dataKey][]*dataRequest
	requestsByTimeout *Heap // Could be a linkedlist, but more flexible.
	peerStates        map[string]*peerState
}

func newBlockManagerState() *blockManagerState {
	return &blockManagerState{
		requestsByKey:     make(map[dataKey][]*dataRequest),
		requestsByTimeout: NewHeap(),
		peerStates:        make(map[string]*peerState),
	}
}

type blockManagerStateJSON struct {
	LastBlockHeight   uint64 // Last contiguous header height
	OtherBlockHeights map[uint64]struct{}
}

func (bms *BlockManagerState) loadState(db _db.LevelDB) {
	bms.mtx.Lock()
	defer bms.mtx.Unlock()
	stateBytes := db.Get(dbKeyState)
	if stateBytes == nil {
		log.Info("New BlockManager with no state")
	} else {
		bmsJSON := &blockManagerStateJSON{}
		err := json.Unmarshal(stateBytes, bmsJSON)
		if err != nil {
			Panicf("Could not unmarshal state bytes: %X", stateBytes)
		}
		bms.lastBlockHeight = bmsJSON.LastBlockHeight
		bms.otherBlockHeights = bmsJSON.OtherBlockHeights
	}
}

func (bms *BlockManagerState) saveState(db _db.LevelDB) {
	bms.mtx.Lock()
	defer bms.mtx.Unlock()
	bmsJSON := &blockManagerStateJSON{
		LastBlockHeight:   bms.lastBlockHeight,
		OtherBlockHeights: bms.otherBlockHeights,
	}
	stateBytes, err := json.Marshal(bmsJSON)
	if err != nil {
		panic("Could not marshal state bytes")
	}
	db.Set(dbKeyState, stateBytes)
}

func (bms *blockManagerState) makeStateMessage() *stateMessage {
	bms.mtx.Lock()
	defer bms.mtx.Unlock()
	return &stateMessage{
		lastBlockHeight: UInt64(bms.lastBlockHeight),
	}
}

func (bms *blockManagerState) createEntryForPeer(peer *peer) {
	bms.mtx.Lock()
	defer bms.mtx.Unlock()
	bms.peerStates[peer.Key] = &peerState{peer: peer}
}

func (bms *blockManagerState) deleteEntryForPeer(peer *peer) {
	bms.mtx.Lock()
	defer bms.mtx.Unlock()
	delete(bms.peerStates, peer.Key)
}

func (bms *blockManagerState) getPeerState(peer *Peer) {
	bms.mtx.Lock()
	defer bms.mtx.Unlock()
	return bms.peerStates[peer.Key]
}

func (bms *blockManagerState) addDataRequest(newRequest *dataRequest) {
	ps.mtx.Lock()
	bms.requestsByKey[newRequest.key] = append(bms.requestsByKey[newRequest.key], newRequest)
	bms.requestsByTimeout.Push(newRequest) // XXX
	peerState, ok := bms.peerStates[newRequest.peer.Key]
	ps.mtx.Unlock()
	if ok {
		peerState.addDataRequest(newRequest)
	}
}

func (bms *blockManagerState) didGetDataFromPeer(key dataKey, peer *p2p.Peer) (canceled []*dataRequest, newHeight bool) {
	bms.mtx.Lock()
	defer bms.mtx.Unlock()
	if key.dataType != dataTypeBlock {
		Panicf("Unknown datatype %X", key.dataType)
	}
	// Adjust lastBlockHeight/otherBlockHeights.
	height := key.height
	if bms.lastBlockHeight == height-1 {
		bms.lastBlockHeight = height
		height++
		for _, ok := bms.otherBlockHeights[height]; ok; {
			delete(bms.otherBlockHeights, height)
			bms.lastBlockHeight = height
			height++
		}
		newHeight = true
	}
	// Remove dataRequests
	requests := bms.requestsByKey[key]
	for _, request := range requests {
		peerState, ok := bms.peerStates[peer.Key]
		if ok {
			peerState.removeDataRequest(request)
		}
		if request.peer == peer {
			continue
		}
		canceled = append(canceled, request)
	}
	delete(bms.requestsByKey, key)

	return canceled, newHeight
}

// Returns at most maxRequestAheadBlock dataKeys that we don't yet have &
// aren't already requesting from maxRequestsPerData peers.
func (bms *blockManagerState) nextWantedKeysForType(dataType byte) []dataKey {
	bms.mtx.Lock()
	defer bms.mtx.Unlock()
	var keys []dataKey
	switch dataType {
	case dataTypeBlock:
		for h := bms.lastBlockHeight + 1; h <= bms.lastBlockHeight+maxRequestAheadBlock; h++ {
			if _, ok := bms.otherBlockHeights[h]; !ok {
				key := newDataKey(dataTypeBlock, h)
				if len(bms.requestsByKey[key]) < maxRequestsPerData {
					keys = append(keys, key)
				}
			}
		}
		return keys
	default:
		Panicf("Unknown datatype %X", dataType)
		return
	}
}

//-----------------------------------------------------------------------------

// dataRequest keeps track of each request for a given peice of data & peer.
type dataRequest struct {
	peer    *p2p.Peer
	key     dataKey
	time    time.Time
	timeout time.Time
}

//-----------------------------------------------------------------------------

type peerState struct {
	mtx             sync.Mutex
	peer            *Peer
	lastBlockHeight uint64         // Last contiguous header height
	requests        []*dataRequest // Active requests
	// XXX we need to
}

// Returns which dataTypes are new as declared by stateMessage.
func (ps *peerState) applyStateMessage(msg *stateMessage) []byte {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	var newTypes []byte
	if uint64(msg.lastBlockHeight) > ps.lastBlockHeight {
		newTypes = append(newTypes, dataTypeBlock)
		ps.lastBlockHeight = uint64(msg.lastBlockHeight)
	} else {
		log.Info("Strange, peer declares a regression of %X", dataTypeBlock)
	}
	return newTypes
}

func (ps *peerState) hasData(key dataKey) bool {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	switch key.dataType {
	case dataTypeBlock:
		return key.height <= ps.lastBlockHeight
	default:
		Panicf("Unknown datatype %X", dataType)
		return false // should not happen
	}
}

func (ps *peerState) addDataRequest(newRequest *dataRequest) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	for _, request := range ps.requests {
		if request.key == newRequest.key {
			return
		}
	}
	ps.requests = append(ps.requests, newRequest)
	return newRequest
}

func (ps *peerState) remoteDataRequest(key dataKey) bool {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	filtered := []*dataRequest{}
	removed := false
	for _, request := range ps.requests {
		if request.key == key {
			removed = true
		} else {
			filtered = append(filtered, request)
		}
	}
	ps.requests = filtered
	return removed
}

func (ps *peerState) canRequestMore() bool {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	return len(ps.requests) < maxRequestsPerPeer
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
	key   dataKey
	type_ Byte
}

const (
	requestTypeFetch    = Byte(0x01)
	requestTypeCanceled = Byte(0x02)
	requestTypeTryAgain = Byte(0x03)
)

func readRequestMessage(r io.Reader) *requestMessage {
	return &requestMessage{
		key:   ReadDataKey(r),
		type_: ReadByte(r),
	}
}

func (m *requestMessage) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteTo(msgTypeRequest, w, n, err)
	n, err = WriteTo(m.key, w, n, err)
	n, err = WriteTo(m.type_, w, n, err)
	return
}

func (m *requestMessage) String() string {
	switch m.type_ {
	case requestTypeByte:
		return fmt.Sprintf("[Request(fetch) %v]", m.key)
	case requestTypeCanceled:
		return fmt.Sprintf("[Request(canceled) %v]", m.key)
	case requestTypeTryAgain:
		return fmt.Sprintf("[Request(tryagain) %v]", m.key)
	default:
		return fmt.Sprintf("[Request(invalid) %v]", m.key)
	}
}

/*
A dataMessage contains block data, maybe requested.
The data can be a Validation, Txs, or whole Block object.
*/
type dataMessage struct {
	key   dataKey
	bytes ByteSlice
}

func readDataMessage(r io.Reader) *dataMessage {
	return &dataMessage{
		key:   readDataKey(r),
		bytes: readByteSlice(r),
	}
}

func (m *dataMessage) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteTo(msgTypeData, w, n, err)
	n, err = WriteTo(m.key, w, n, err)
	n, err = WriteTo(m.bytes, w, n, err)
	return
}

func (m *dataMessage) String() string {
	return fmt.Sprintf("[Data %v]", m.key)
}
