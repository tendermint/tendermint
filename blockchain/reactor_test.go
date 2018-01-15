package blockchain

import (
	"testing"

	wire "github.com/tendermint/go-wire"
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
	blockStore := NewBlockStore(dbm.NewMemDB())
	state, _ := sm.LoadStateFromDBOrGenesisFile(dbm.NewMemDB(), config.GenesisFile())
	return state, blockStore
}

func newBlockchainReactor(logger log.Logger, maxBlockHeight int64) *BlockchainReactor {
	state, blockStore := makeStateAndBlockStore(logger)

	// Make the blockchainReactor itself
	fastSync := true
	var nilApp proxy.AppConnConsensus
	blockExec := sm.NewBlockExecutor(dbm.NewMemDB(), log.TestingLogger(), nilApp, types.MockMempool{}, types.MockEvidencePool{})

	bcReactor := NewBlockchainReactor(state.Copy(), blockExec, blockStore, fastSync)
	bcReactor.SetLogger(logger.With("module", "blockchain"))

	// Next: we need to set a switch in order for peers to be added in
	bcReactor.Switch = p2p.NewSwitch(cfg.DefaultP2PConfig())

	// Lastly: let's add some blocks in
	for blockHeight := int64(1); blockHeight <= maxBlockHeight; blockHeight++ {
		firstBlock := makeBlock(blockHeight, state)
		secondBlock := makeBlock(blockHeight+1, state)
		firstParts := firstBlock.MakePartSet(state.ConsensusParams.BlockGossip.BlockPartSizeBytes)
		blockStore.SaveBlock(firstBlock, firstParts, secondBlock.LastCommit)
	}

	return bcReactor
}

func TestNoBlockMessageResponse(t *testing.T) {
	maxBlockHeight := int64(20)

	bcr := newBlockchainReactor(log.TestingLogger(), maxBlockHeight)
	bcr.Start()
	defer bcr.Stop()

	// Add some peers in
	peer := newbcrTestPeer(cmn.RandStr(12))
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
	// wait to hear response
	for _, tt := range tests {
		reqBlockMsg := &bcBlockRequestMessage{tt.height}
		reqBlockBytes, err := wire.MarshalBinary(struct{ BlockchainMessage }{reqBlockMsg})
		if err != nil {
			t.Fatal(err)
		}
		bcr.Receive(chID, peer, reqBlockBytes)
		value := peer.lastValue()
		msg := value.(struct{ BlockchainMessage }).BlockchainMessage

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
	cmn.Service
	key string
	ch  chan interface{}
}

var _ p2p.Peer = (*bcrTestPeer)(nil)

func newbcrTestPeer(key string) *bcrTestPeer {
	return &bcrTestPeer{
		Service: cmn.NewBaseService(nil, "bcrTestPeer", nil),
		key:     key,
		ch:      make(chan interface{}, 2),
	}
}

func (tp *bcrTestPeer) lastValue() interface{} { return <-tp.ch }

func (tp *bcrTestPeer) TrySend(chID byte, value interface{}) bool {
	if _, ok := value.(struct{ BlockchainMessage }).BlockchainMessage.(*bcStatusResponseMessage); ok {
		// Discard status response messages since they skew our results
		// We only want to deal with:
		// + bcBlockResponseMessage
		// + bcNoBlockResponseMessage
	} else {
		tp.ch <- value
	}
	return true
}

func (tp *bcrTestPeer) Send(chID byte, data interface{}) bool { return tp.TrySend(chID, data) }
func (tp *bcrTestPeer) NodeInfo() *p2p.NodeInfo               { return nil }
func (tp *bcrTestPeer) Status() p2p.ConnectionStatus          { return p2p.ConnectionStatus{} }
func (tp *bcrTestPeer) Key() string                           { return tp.key }
func (tp *bcrTestPeer) IsOutbound() bool                      { return false }
func (tp *bcrTestPeer) IsPersistent() bool                    { return true }
func (tp *bcrTestPeer) Get(s string) interface{}              { return s }
func (tp *bcrTestPeer) Set(string, interface{})               {}
