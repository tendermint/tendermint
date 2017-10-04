package blockchain

import (
	"bytes"
	"testing"

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/p2p"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
	cmn "github.com/tendermint/tmlibs/common"
	"github.com/tendermint/tmlibs/db"
	"github.com/tendermint/tmlibs/log"
)

func newBlockchainReactor(logger log.Logger, maxBlockHeight int) *BlockchainReactor {
	config := cfg.ResetTestRoot("node_node_test")

	blockStoreDB := db.NewDB("blockstore", config.DBBackend, config.DBDir())
	blockStore := NewBlockStore(blockStoreDB)

	stateLogger := logger.With("module", "state")

	// Get State
	stateDB := db.NewDB("state", config.DBBackend, config.DBDir())
	state := sm.GetState(stateDB, config.GenesisFile())
	state.SetLogger(stateLogger)
	state.Save()

	// Make the blockchainReactor itself
	fastSync := true
	bcReactor := NewBlockchainReactor(state.Copy(), nil, blockStore, fastSync)

	// Next: we need to set a switch in order for peers to be added in
	bcReactor.Switch = p2p.NewSwitch(cfg.DefaultP2PConfig())
	bcReactor.SetLogger(logger.With("module", "blockchain"))

	// Lastly: let's add some blocks in
	for blockHeight := 1; blockHeight <= maxBlockHeight; blockHeight++ {
		firstBlock := makeBlock(blockHeight, state)
		secondBlock := makeBlock(blockHeight+1, state)
		firstParts := firstBlock.MakePartSet(types.DefaultBlockPartSize)
		blockStore.SaveBlock(firstBlock, firstParts, secondBlock.LastCommit)
	}

	return bcReactor
}

func newbcrTestPeer(key string) *bcrTestPeer {
	return &bcrTestPeer{
		key: key,
		kvm: make(map[byte]interface{}),
		ch:  make(chan *keyValue, 2),
	}
}

func TestNoBlockMessageResponse(t *testing.T) {
	logBuf := new(bytes.Buffer)
	logger := log.NewTMLogger(logBuf)
	maxBlockHeight := 20

	bcr := newBlockchainReactor(logger, maxBlockHeight)
	go bcr.OnStart()
	defer bcr.Stop()

	// Add some peers in
	peer := newbcrTestPeer(cmn.RandStr(12))
	bcr.AddPeer(peer)

	chID := byte(0x01)
	
	tests := [...]struct {
		height   int
		existent bool
	}{
		0: {
			height:   maxBlockHeight + 2,
			existent: false,
		},
		1: {
			height:   10,
			existent: true, // We should have this
		},
		2: {
			height:   1,
			existent: true, // We should have this
		},
		3: {
			height:   100,
			existent: false,
		},
	}

	// Currently the repsonses take asynchronous paths so
	// the results are skewed. However, we can ensure that
	// the (expectedHeight, expectedExistence) all tally up.
	heightTally := map[int]bool{}
	var results []BlockchainMessage

	msgRequest := byte(0x10)
	for _, tt := range tests {
		reqBlockBytes := []byte{msgRequest, 0x02, 0x00, byte(tt.height)}
		bcr.Receive(chID, peer, reqBlockBytes)
		heightTally[tt.height] = tt.existent
		kv := peer.lastKeyValue()
		results = append(results, kv.value.(struct{ BlockchainMessage }).BlockchainMessage)
	}

	for i, res := range results {
		if bcRM, ok := res.(*bcBlockResponseMessage); ok {
			block := bcRM.Block
			if block == nil {
				t.Errorf("result: #%d: expecting a non-nil block", i)
				continue
			}
			mustExist, foundHeight := heightTally[block.Height]
			if !foundHeight {
				t.Errorf("result: #%d, missing height %d", i, block.Height)
				continue
			}
			if !mustExist {
				t.Errorf("result: #%d mustExist", i)
			} else {
				delete(heightTally, block.Height)
			}
		} else if bcNBRSM, ok := res.(*bcNoBlockResponseMessage); ok {
			mustExist, foundHeight := heightTally[bcNBRSM.Height]
			if !foundHeight {
				t.Errorf("result: #%d, missing height %d", i, bcNBRSM.Height)
				continue
			}
			if mustExist {
				t.Errorf("result: #%d mustNotExist", i)
			} else {
				// Passes, remove this height from the tally
				delete(heightTally, bcNBRSM.Height)
			}
		} else {
			t.Errorf("result: #%d is an unexpected type: %T; data: %#v", i, res, res)
		}
	}

	if len(heightTally) > 0 {
		t.Errorf("Untallied heights: %#v\n", heightTally)
	}
}

func makeTxs(blockNumber int) (txs []types.Tx) {
	for i := 0; i < 10; i++ {
		txs = append(txs, types.Tx([]byte{byte(blockNumber)}))
	}
	return txs
}

const testPartSize = 65536

func makeBlock(blockNumber int, state *sm.State) *types.Block {
	prevHash := state.LastBlockID.Hash
	prevParts := types.PartSetHeader{}
	valHash := state.Validators.Hash()
	prevBlockID := types.BlockID{prevHash, prevParts}
	block, _ := types.MakeBlock(blockNumber, "test_chain", makeTxs(blockNumber),
		new(types.Commit), prevBlockID, valHash, state.AppHash, testPartSize)
	return block
}

// The Test peer
type bcrTestPeer struct {
	key string
	ch  chan *keyValue
	kvm map[byte]interface{}
}

type keyValue struct {
	key   interface{}
	value interface{}
}

var _ p2p.Peer = (*bcrTestPeer)(nil)

func (tp *bcrTestPeer) Key() string        { return tp.key }
func (tp *bcrTestPeer) IsOutbound() bool   { return false }
func (tp *bcrTestPeer) IsPersistent() bool { return true }
func (tp *bcrTestPeer) IsRunning() bool    { return true }

func (tp *bcrTestPeer) OnStop() {}

func (tp *bcrTestPeer) OnReset() error         { return nil }
func (tp *bcrTestPeer) Reset() (bool, error)   { return false, nil }
func (tp *bcrTestPeer) OnStart() error         { return nil }
func (tp *bcrTestPeer) SetLogger(l log.Logger) {}

func (tp *bcrTestPeer) Start() (bool, error) { return true, nil }
func (tp *bcrTestPeer) Stop() bool           { return true }
func (tp *bcrTestPeer) String() string       { return tp.key }

func (tp *bcrTestPeer) TrySend(chID byte, value interface{}) bool {
	if _, ok := value.(struct{ BlockchainMessage }).BlockchainMessage.(*bcStatusResponseMessage); ok {
		// Discard status response messages since they skew our results
		// We only want to deal with:
		// + bcBlockResponseMessage
		// + bcNoBlockResponseMessage
	} else {
		tp.ch <- &keyValue{key: chID, value: value}
	}
	return true
}

func (tp *bcrTestPeer) Send(chID byte, data interface{}) bool {
	return tp.TrySend(chID, data)
}

func (tp *bcrTestPeer) NodeInfo() *p2p.NodeInfo      { return nil }
func (tp *bcrTestPeer) Status() p2p.ConnectionStatus { return p2p.ConnectionStatus{} }

func (tp *bcrTestPeer) Get(s string) interface{} { return s }
func (tp *bcrTestPeer) Set(string, interface{})  {}
func (tp *bcrTestPeer) lastKeyValue() *keyValue  { return <-tp.ch }
