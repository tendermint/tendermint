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

	dbKeyState = "state"
)

//-----------------------------------------------------------------------------

// We request each item separately.

const (
	dataTypeHeader     = byte(0x01)
	dataTypeValidation = byte(0x02)
	dataTypeTxs        = byte(0x03)
)

func _dataKey(dataType byte, height int) {
	switch dataType {
	case dataTypeHeader:
		return fmt.Sprintf("H%v", height)
	case dataTypeValidation:
		return fmt.Sprintf("V%v", height)
	case dataTypeTxs:
		return fmt.Sprintf("T%v", height)
	default:
		panic("Unknown datatype %X", dataType)
	}
}

func dataTypeFromObj(data interface{}) {
	switch data.(type) {
	case *Header:
		return dataTypeHeader
	case *Validation:
		return dataTypeValidation
	case *Txs:
		return dataTypeTxs
	default:
		panic("Unexpected datatype: %v", data)
	}
}

//-----------------------------------------------------------------------------

// TODO: document
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
func (bm *BlockManager) StoreData(dataObj interface{}) {
	bm.mtx.Lock()
	defer bm.mtx.Unlock()
	dataType := dataTypeForObj(dataObj)
	dataKey := _dataKey(dataType, dataObj)
	// Update state
	// TODO
	// Remove dataState entry, we'll no longer request this.
	_dataState := bm.dataStates[dataKey]
	removedRequests := _dataState.removeRequestsForDataType(dataType)
	for _, request := range removedRequests {
		// TODO in future, notify peer that the request has been canceled.
		// No point doing this yet, requests in blocksCh are handled singlethreaded.
	}
	// What are we doing here?
	_peerState := bm.peerstates[dataKey]

	// If we have new data that extends our contiguous range, then announce it.
}

func (bm *BlockManager) LoadData(dataType byte, height int) interface{} {
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
			bm.peerStates[event.Peer.RemoteAddress().String()] = &peerState{}
			// Share our state with event.Peer
			msg := &stateMessage{
				lastHeaderHeight:     bm.state.lastHeaderHeight,
				lastValidationHeight: bm.state.lastValidationHeight,
				lastTxsHeight:        bm.state.lastTxsHeight,
			}
			tm := p2p.TypedMessage{msgTypeRequest, msg}
			event.Peer.TrySend(NewPacket(blocksCh, tm))
		case p2p.SwitchEventDonePeer:
			// Remove entry from .peerStates
			delete(bm.peerStates, event.Peer.RemoteAddress().String())
		default:
			log.Warning("Unhandled switch event type")
		}
	}
}

// Handle requests from the blocks channel
func (bm *BlockManager) requestsHandler() {
	for {
		inPkt := bm.sw.Receive(blocksCh) // {Peer, Time, Packet}
		if inPkt == nil {
			// Client has stopped
			break
		}

		// decode message
		msg := decodeMessage(inPkt.Bytes)
		log.Info("requestHandler received %v", msg)

		switch msg.(type) {
		case *stateMessage:
			m := msg.(*stateMessage)
			peerState := bm.peerStates[inPkt.Peer.RemoteAddress.String()]
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
			// See if we want the data.
			// Validate data.
			// Add to db.
			// Update state & broadcast as necessary.
		default:
			// Ignore unknown message
			// bm.sw.StopPeerForError(inPkt.Peer, errInvalidMessage)
		}
	}

	// Cleanup
}

//-----------------------------------------------------------------------------

// blockManagerState keeps track of which block parts are stored locally.
// It's also persisted via JSON in the db.
type blockManagerState struct {
	lastHeaderHeight       uint64 // Last contiguous header height
	lastValidationHeight   uint64 // Last contiguous validation height
	lastTxsHeight          uint64 // Last contiguous txs height
	otherHeaderHeights     []uint64
	otherValidationHeights []uint64
	otherTxsHeights        []uint64
}

//-----------------------------------------------------------------------------

// dataRequest keeps track of each request for a given peice of data & peer.
type dataRequest struct {
	peer     *p2p.Peer
	dataType byte
	height   uint64
	time     time.Time
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

// XXX
type peerState struct {
	mtx                  sync.Mutex
	lastHeaderHeight     uint64 // Last contiguous header height
	lastValidationHeight uint64 // Last contiguous validation height
	lastTxsHeight        uint64 // Last contiguous txs height
	dataBytesSent        uint64 // Data bytes sent to peer
	dataBytesReceived    uint64 // Data bytes received from peer
	numItemsReceived     uint64 // Number of data items received
	numItemsUnreceived   uint64 // Number of data items requested but not received
	numItemsSent         uint64 // Number of data items sent
	requests             map[string]*dataRequest
}

func (ps *peerState) applyStateMessage(msg *stateMessage) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	ps.lastHeaderHeight = msg.lastHeaderHeight
	ps.lastValidationHeight = msg.lastValidationHeight
	ps.lastTxsHeight = msg.lastTxsHeight
}

// Call this function for each data item received from peer, if the item was requested.
// If the request timed out, dataBytesReceived is set to 0 to denote failure.
func (ps *peerState) didReceiveData(dataKey string, dataBytesReceived uint64) {
	ps.mtx.Lock()
	defer ps.mtx.Lock()
	request := ps.requests[dataKey]
	if request == nil {
		log.Warning("Could not find peerState request with dataKey %v", dataKey)
		return
	}
	if dataBytesReceived == 0 {
		ps.numItemsUnreceived += 1
	} else {
		ps.dataBytesReceived += dataBytesReceived
		ps.numItemsReceived += 1
	}
	delete(ps.requests, dataKey)
}

// Call this function for each data item sent to peer, if the item was requested.
func (ps *peerState) didSendData(dataKey string, dataBytesSent uint64) {
	ps.mtx.Lock()
	defer ps.mtx.Lock()
	if dataBytesSent == 0 {
		log.Warning("didSendData expects dataBytesSent > 0")
		return
	}
	ps.dataBytesSent += dataBytesSent
	ps.numItemsSent += 1
}

//-----------------------------------------------------------------------------

/* Messages */

// TODO: check for unnecessary extra bytes at the end.
func decodeMessage(bz ByteSlice) (msg Message) {
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
	lastHeaderHeight     uint64 // Last contiguous header height
	lastValidationHeight uint64 // Last contiguous validation height
	lastTxsHeight        uint64 // Last contiguous txs height
}

func readStateMessage(r io.Reader) *stateMessage {
	lastHeaderHeight := ReadUInt64(r)
	lastValidationHeight := ReadUInt64(r)
	lastTxsHeight := ReadUInt64(r)
	return &stateMessage{
		lastHeaderHeight:     lastHeaderHeight,
		lastValidationHeight: lastValidationHeight,
		lastTxsHeight:        lastTxsHeight,
	}
}

func (m *stateMessage) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteTo(msgTypeState, w, n, err)
	n, err = WriteTo(m.lastHeaderHeight, w, n, err)
	n, err = WriteTo(m.lastValidationHeight, w, n, err)
	n, err = WriteTo(m.lastTxsHeight, w, n, err)
	return
}

func (m *stateMessage) String() string {
	return fmt.Sprintf("[State %v/%v/%v]",
		m.lastHeaderHeight, m.lastValidationHeight, m.lastTxsHeight)
}

/*
A requestMessage requests a block and/or header at a given height.
*/
type requestMessage struct {
	dataType Byte
	height   UInt64
}

func readRequestMessage(r io.Reader) *requestMessage {
	requestType := ReadByte(r)
	height := ReadUInt64(r)
	return &requestMessage{
		dataType: requestType,
		height:   height,
	}
}

func (m *requestMessage) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteTo(msgTypeRequest, w, n, err)
	n, err = WriteTo(m.dataType, w, n, err)
	n, err = WriteTo(m.height, w, n, err)
	return
}

func (m *requestMessage) String() string {
	return fmt.Sprintf("[Request %X@%v]", m.dataType, m.height)
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
