package v2

import (
	"net"
	"testing"

	"github.com/tendermint/tendermint/behaviour"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/p2p/conn"
	"github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

/*
# What do we test in v1?

* TestFastSyncNoBlockResponse
	* test that we switch to consensus after not receiving a block for a certain amount of time
* TestFastSyncBadBlockStopsPeer
	* test that a bad block will evict the peer
* TestBcBlockRequestMessageValidateBasic
	* test the behaviour of bcBlockRequestMessage.ValidateBasic()
* TestBcBlockResponseMessageValidateBasic
	* test the validation of of the message body

# What do we want to test:
	* Initialization from disk store
	* BlockResponse from state
		* Invalid Request
		* Block Missing
		* Block Found
	* StatusRequest
		* Invalid Message
		* Valid Status
	* Termination
		* Timeout
		* Completion on sync blocks
*/

type mockPeer struct {
	cmn.Service
}

func (mp mockPeer) FlushStop()           {}
func (mp mockPeer) ID() p2p.ID           { return "foo" }
func (mp mockPeer) RemoteIP() net.IP     { return net.IP{} }
func (mp mockPeer) RemoteAddr() net.Addr { return &net.TCPAddr{IP: mp.RemoteIP(), Port: 8800} }

func (mp mockPeer) IsOutbound() bool   { return true }
func (mp mockPeer) IsPersistent() bool { return true }
func (mp mockPeer) CloseConn() error   { return nil }

func (mp mockPeer) NodeInfo() p2p.NodeInfo {
	return p2p.DefaultNodeInfo{
		ID_:        "",
		ListenAddr: "",
	}
}
func (mp mockPeer) Status() conn.ConnectionStatus { return conn.ConnectionStatus{} }
func (mp mockPeer) SocketAddr() *p2p.NetAddress   { return &p2p.NetAddress{} }

func (mp mockPeer) Send(byte, []byte) bool    { return true }
func (mp mockPeer) TrySend(byte, []byte) bool { return true }

func (mp mockPeer) Set(string, interface{}) {}
func (mp mockPeer) Get(string) interface{}  { return struct{}{} }

type mockBlockStore struct {
	blocks map[int64]*types.Block
}

func (ml *mockBlockStore) LoadBlock(height int64) *types.Block {
	return ml.blocks[height]
}

func (ml *mockBlockStore) SaveBlock(block *types.Block, part *types.PartSet, commit *types.Commit) {
	ml.blocks[block.Height] = block
}

type mockBlockApplier struct {
}

// XXX: Add whitelist/blacklist?
func (mba *mockBlockApplier) ApplyBlock(state state.State, blockID types.BlockID, block *types.Block) (state.State, error) {
	return state, nil
}

func TestReactor(t *testing.T) {
	var (
		chID       = byte(0x40)
		bufferSize = 10
		loader     = &mockBlockStore{}
		applier    = &mockBlockApplier{}
		state      = state.State{}
		peer       = mockPeer{}
		reporter   = behaviour.NewMockReporter()
		reactor    = NewReactor(state, loader, reporter, applier, bufferSize)
	)

	// How do we serialize events to work with Receive?
	reactor.Start()
	script := [][]byte{
		cdc.MustMarshalBinaryBare(&bcBlockRequestMessage{Height: 1}),
	}

	for _, event := range script {
		reactor.Receive(chID, peer, event)
	}
	reactor.Stop()
}
