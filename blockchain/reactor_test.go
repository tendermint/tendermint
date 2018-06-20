package blockchain

import (
	"net"
	"testing"

	cmn "github.com/tendermint/tmlibs/common"
	dbm "github.com/tendermint/tmlibs/db"
	"github.com/tendermint/tmlibs/log"

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/proxy"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

func makeStateAndBlockStore(logger log.Logger) (sm.State, *BlockStore) {
	config := cfg.ResetTestRoot("blockchain_reactor_test")
	// blockDB := dbm.NewDebugDB("blockDB", dbm.NewMemDB())
	// stateDB := dbm.NewDebugDB("stateDB", dbm.NewMemDB())
	blockDB := dbm.NewMemDB()
	stateDB := dbm.NewMemDB()
	blockStore := NewBlockStore(blockDB)
	state, err := sm.LoadStateFromDBOrGenesisFile(stateDB, config.GenesisFile())
	if err != nil {
		panic(cmn.ErrorWrap(err, "error constructing state from genesis file"))
	}
	return state, blockStore
}

func newBlockchainReactor(logger log.Logger, maxBlockHeight int64) *BlockchainReactor {
	state, blockStore := makeStateAndBlockStore(logger)

	// Make the blockchainReactor itself
	fastSync := true
	var nilApp proxy.AppConnConsensus
	blockExec := sm.NewBlockExecutor(dbm.NewMemDB(), log.TestingLogger(), nilApp,
		sm.MockMempool{}, sm.MockEvidencePool{})

	bcReactor := NewBlockchainReactor(state.Copy(), blockExec, blockStore, fastSync)
	bcReactor.SetLogger(logger.With("module", "blockchain"))

	// Next: we need to set a switch in order for peers to be added in
	bcReactor.Switch = p2p.NewSwitch(cfg.DefaultP2PConfig(), nil)

	// Lastly: let's add some blocks in
	for blockHeight := int64(1); blockHeight <= maxBlockHeight; blockHeight++ {
		firstBlock := makeBlock(blockHeight, state)
		secondBlock := makeBlock(blockHeight+1, state)
		firstParts := firstBlock.MakePartSet(state.ConsensusParams.BlockGossip.BlockPartSizeBytes)
		blockStore.SaveBlock(firstBlock, firstParts, secondBlock.LastCommit)
	}

	return bcReactor
}

func TestNoBlockResponse(t *testing.T) {
	maxBlockHeight := int64(20)

	bcr := newBlockchainReactor(log.TestingLogger(), maxBlockHeight)
	bcr.Start()
	defer bcr.Stop()

	// Add some peers in
	peer := newbcrTestPeer(p2p.ID(cmn.RandStr(12)))
	bcr.AddPeer(peer)

	chID := byte(0x01)

	tests := []struct {
		height   int64
		existent bool
	}{
		{maxBlockHeight + 2, false},
		{10, true},
		{1, true},
		{100, false},
	}

	// receive a request message from peer,
	// wait for our response to be received on the peer
	for _, tt := range tests {
		reqBlockMsg := &bcBlockRequestMessage{tt.height}
		reqBlockBytes := cdc.MustMarshalBinaryBare(reqBlockMsg)
		bcr.Receive(chID, peer, reqBlockBytes)
		msg := peer.lastBlockchainMessage()

		if tt.existent {
			if blockMsg, ok := msg.(*bcBlockResponseMessage); !ok {
				t.Fatalf("Expected to receive a block response for height %d", tt.height)
			} else if blockMsg.Block.Height != tt.height {
				t.Fatalf("Expected response to be for height %d, got %d", tt.height, blockMsg.Block.Height)
			}
		} else {
			if noBlockMsg, ok := msg.(*bcNoBlockResponseMessage); !ok {
				t.Fatalf("Expected to receive a no block response for height %d", tt.height)
			} else if noBlockMsg.Height != tt.height {
				t.Fatalf("Expected response to be for height %d, got %d", tt.height, noBlockMsg.Height)
			}
		}
	}
}

/*
// NOTE: This is too hard to test without
// an easy way to add test peer to switch
// or without significant refactoring of the module.
// Alternatively we could actually dial a TCP conn but
// that seems extreme.
func TestBadBlockStopsPeer(t *testing.T) {
	maxBlockHeight := int64(20)

	bcr := newBlockchainReactor(log.TestingLogger(), maxBlockHeight)
	bcr.Start()
	defer bcr.Stop()

	// Add some peers in
	peer := newbcrTestPeer(p2p.ID(cmn.RandStr(12)))

	// XXX: This doesn't add the peer to anything,
	// so it's hard to check that it's later removed
	bcr.AddPeer(peer)
	assert.True(t, bcr.Switch.Peers().Size() > 0)

	// send a bad block from the peer
	// default blocks already dont have commits, so should fail
	block := bcr.store.LoadBlock(3)
	msg := &bcBlockResponseMessage{Block: block}
	peer.Send(BlockchainChannel, struct{ BlockchainMessage }{msg})

	ticker := time.NewTicker(time.Millisecond * 10)
	timer := time.NewTimer(time.Second * 2)
LOOP:
	for {
		select {
		case <-ticker.C:
			if bcr.Switch.Peers().Size() == 0 {
				break LOOP
			}
		case <-timer.C:
			t.Fatal("Timed out waiting to disconnect peer")
		}
	}
}
*/

//----------------------------------------------
// utility funcs

func makeTxs(height int64) (txs []types.Tx) {
	for i := 0; i < 10; i++ {
		txs = append(txs, types.Tx([]byte{byte(height), byte(i)}))
	}
	return txs
}

func makeBlock(height int64, state sm.State) *types.Block {
	block, _ := state.MakeBlock(height, makeTxs(height), new(types.Commit))
	return block
}

// The Test peer
type bcrTestPeer struct {
	cmn.BaseService
	id p2p.ID
	ch chan interface{}
}

var _ p2p.Peer = (*bcrTestPeer)(nil)

func newbcrTestPeer(id p2p.ID) *bcrTestPeer {
	bcr := &bcrTestPeer{
		id: id,
		ch: make(chan interface{}, 2),
	}
	bcr.BaseService = *cmn.NewBaseService(nil, "bcrTestPeer", bcr)
	return bcr
}

func (tp *bcrTestPeer) lastBlockchainMessage() interface{} { return <-tp.ch }

func (tp *bcrTestPeer) TrySend(chID byte, msgBytes []byte) bool {
	var msg BlockchainMessage
	err := cdc.UnmarshalBinaryBare(msgBytes, &msg)
	if err != nil {
		panic(cmn.ErrorWrap(err, "Error while trying to parse a BlockchainMessage"))
	}
	if _, ok := msg.(*bcStatusResponseMessage); ok {
		// Discard status response messages since they skew our results
		// We only want to deal with:
		// + bcBlockResponseMessage
		// + bcNoBlockResponseMessage
	} else {
		tp.ch <- msg
	}
	return true
}

func (tp *bcrTestPeer) Send(chID byte, msgBytes []byte) bool { return tp.TrySend(chID, msgBytes) }
func (tp *bcrTestPeer) NodeInfo() p2p.NodeInfo               { return p2p.NodeInfo{} }
func (tp *bcrTestPeer) Status() p2p.ConnectionStatus         { return p2p.ConnectionStatus{} }
func (tp *bcrTestPeer) ID() p2p.ID                           { return tp.id }
func (tp *bcrTestPeer) IsOutbound() bool                     { return false }
func (tp *bcrTestPeer) IsPersistent() bool                   { return true }
func (tp *bcrTestPeer) Get(s string) interface{}             { return s }
func (tp *bcrTestPeer) Set(string, interface{})              {}
func (tp *bcrTestPeer) RemoteIP() net.IP                     { return []byte{127, 0, 0, 1} }
