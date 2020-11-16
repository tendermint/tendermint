package v1

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/types"
)

type testPeer struct {
	id     p2p.ID
	base   int64
	height int64
}

type testBcR struct {
	logger log.Logger
}

type testValues struct {
	numRequestsSent int
}

var testResults testValues

func resetPoolTestResults() {
	testResults.numRequestsSent = 0
}

func (testR *testBcR) sendPeerError(err error, peerID p2p.ID) {
}

func (testR *testBcR) sendStatusRequest() {
}

func (testR *testBcR) sendBlockRequest(peerID p2p.ID, height int64) error {
	testResults.numRequestsSent++
	return nil
}

func (testR *testBcR) resetStateTimer(name string, timer **time.Timer, timeout time.Duration) {
}

func (testR *testBcR) switchToConsensus() {

}

func newTestBcR() *testBcR {
	testBcR := &testBcR{logger: log.TestingLogger()}
	return testBcR
}

type tPBlocks struct {
	id     p2p.ID
	create bool
}

// Makes a block pool with specified current height, list of peers, block requests and block responses
func makeBlockPool(bcr *testBcR, height int64, peers []BpPeer, blocks map[int64]tPBlocks) *BlockPool {
	bPool := NewBlockPool(height, bcr)
	bPool.SetLogger(bcr.logger)

	txs := []types.Tx{types.Tx("foo"), types.Tx("bar")}

	var maxH int64
	for _, p := range peers {
		if p.Height > maxH {
			maxH = p.Height
		}
		bPool.peers[p.ID] = NewBpPeer(p.ID, p.Base, p.Height, bcr.sendPeerError, nil)
		bPool.peers[p.ID].SetLogger(bcr.logger)

	}

	coreChainLock := types.NewMockChainLock(1)

	bPool.MaxPeerHeight = maxH
	for h, p := range blocks {
		bPool.blocks[h] = p.id
		bPool.peers[p.id].RequestSent(h)
		if p.create {
			// simulate that a block at height h has been received
			_ = bPool.peers[p.id].AddBlock(types.MakeBlock(h, coreChainLock.CoreBlockHeight, &coreChainLock, txs, nil, nil), 100)
		}
	}
	return bPool
}

func assertPeerSetsEquivalent(t *testing.T, set1 map[p2p.ID]*BpPeer, set2 map[p2p.ID]*BpPeer) {
	assert.Equal(t, len(set1), len(set2))
	for peerID, peer1 := range set1 {
		peer2 := set2[peerID]
		assert.NotNil(t, peer2)
		assert.Equal(t, peer1.NumPendingBlockRequests, peer2.NumPendingBlockRequests)
		assert.Equal(t, peer1.Height, peer2.Height)
		assert.Equal(t, peer1.Base, peer2.Base)
		assert.Equal(t, len(peer1.blocks), len(peer2.blocks))
		for h, block1 := range peer1.blocks {
			block2 := peer2.blocks[h]
			// block1 and block2 could be nil if a request was made but no block was received
			assert.Equal(t, block1, block2)
		}
	}
}

func assertBlockPoolEquivalent(t *testing.T, poolWanted, pool *BlockPool) {
	assert.Equal(t, poolWanted.blocks, pool.blocks)
	assertPeerSetsEquivalent(t, poolWanted.peers, pool.peers)
	assert.Equal(t, poolWanted.MaxPeerHeight, pool.MaxPeerHeight)
	assert.Equal(t, poolWanted.Height, pool.Height)

}

func TestBlockPoolUpdatePeer(t *testing.T) {
	testBcR := newTestBcR()

	tests := []struct {
		name       string
		pool       *BlockPool
		args       testPeer
		poolWanted *BlockPool
		errWanted  error
	}{
		{
			name:       "add a first short peer",
			pool:       makeBlockPool(testBcR, 100, []BpPeer{}, map[int64]tPBlocks{}),
			args:       testPeer{"P1", 0, 50},
			errWanted:  errPeerTooShort,
			poolWanted: makeBlockPool(testBcR, 100, []BpPeer{}, map[int64]tPBlocks{}),
		},
		{
			name:       "add a first good peer",
			pool:       makeBlockPool(testBcR, 100, []BpPeer{}, map[int64]tPBlocks{}),
			args:       testPeer{"P1", 0, 101},
			poolWanted: makeBlockPool(testBcR, 100, []BpPeer{{ID: "P1", Height: 101}}, map[int64]tPBlocks{}),
		},
		{
			name:       "add a first good peer with base",
			pool:       makeBlockPool(testBcR, 100, []BpPeer{}, map[int64]tPBlocks{}),
			args:       testPeer{"P1", 10, 101},
			poolWanted: makeBlockPool(testBcR, 100, []BpPeer{{ID: "P1", Base: 10, Height: 101}}, map[int64]tPBlocks{}),
		},
		{
			name:       "increase the height of P1 from 120 to 123",
			pool:       makeBlockPool(testBcR, 100, []BpPeer{{ID: "P1", Height: 120}}, map[int64]tPBlocks{}),
			args:       testPeer{"P1", 0, 123},
			poolWanted: makeBlockPool(testBcR, 100, []BpPeer{{ID: "P1", Height: 123}}, map[int64]tPBlocks{}),
		},
		{
			name:       "decrease the height of P1 from 120 to 110",
			pool:       makeBlockPool(testBcR, 100, []BpPeer{{ID: "P1", Height: 120}}, map[int64]tPBlocks{}),
			args:       testPeer{"P1", 0, 110},
			errWanted:  errPeerLowersItsHeight,
			poolWanted: makeBlockPool(testBcR, 100, []BpPeer{}, map[int64]tPBlocks{}),
		},
		{
			name: "decrease the height of P1 from 105 to 102 with blocks",
			pool: makeBlockPool(testBcR, 100, []BpPeer{{ID: "P1", Height: 105}},
				map[int64]tPBlocks{
					100: {"P1", true}, 101: {"P1", true}, 102: {"P1", true}}),
			args:      testPeer{"P1", 0, 102},
			errWanted: errPeerLowersItsHeight,
			poolWanted: makeBlockPool(testBcR, 100, []BpPeer{},
				map[int64]tPBlocks{}),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			pool := tt.pool
			err := pool.UpdatePeer(tt.args.id, tt.args.base, tt.args.height)
			assert.Equal(t, tt.errWanted, err)
			assert.Equal(t, tt.poolWanted.blocks, tt.pool.blocks)
			assertPeerSetsEquivalent(t, tt.poolWanted.peers, tt.pool.peers)
			assert.Equal(t, tt.poolWanted.MaxPeerHeight, tt.pool.MaxPeerHeight)
		})
	}
}

func TestBlockPoolRemovePeer(t *testing.T) {
	testBcR := newTestBcR()

	type args struct {
		peerID p2p.ID
		err    error
	}

	tests := []struct {
		name       string
		pool       *BlockPool
		args       args
		poolWanted *BlockPool
	}{
		{
			name:       "attempt to delete non-existing peer",
			pool:       makeBlockPool(testBcR, 100, []BpPeer{{ID: "P1", Height: 120}}, map[int64]tPBlocks{}),
			args:       args{"P99", nil},
			poolWanted: makeBlockPool(testBcR, 100, []BpPeer{{ID: "P1", Height: 120}}, map[int64]tPBlocks{}),
		},
		{
			name:       "delete the only peer without blocks",
			pool:       makeBlockPool(testBcR, 100, []BpPeer{{ID: "P1", Height: 120}}, map[int64]tPBlocks{}),
			args:       args{"P1", nil},
			poolWanted: makeBlockPool(testBcR, 100, []BpPeer{}, map[int64]tPBlocks{}),
		},
		{
			name: "delete the shortest of two peers without blocks",
			pool: makeBlockPool(
				testBcR,
				100,
				[]BpPeer{{ID: "P1", Height: 100}, {ID: "P2", Height: 120}},
				map[int64]tPBlocks{}),
			args:       args{"P1", nil},
			poolWanted: makeBlockPool(testBcR, 100, []BpPeer{{ID: "P2", Height: 120}}, map[int64]tPBlocks{}),
		},
		{
			name: "delete the tallest of two peers without blocks",
			pool: makeBlockPool(
				testBcR,
				100,
				[]BpPeer{{ID: "P1", Height: 100}, {ID: "P2", Height: 120}},
				map[int64]tPBlocks{}),
			args:       args{"P2", nil},
			poolWanted: makeBlockPool(testBcR, 100, []BpPeer{{ID: "P1", Height: 100}}, map[int64]tPBlocks{}),
		},
		{
			name: "delete the only peer with block requests sent and blocks received",
			pool: makeBlockPool(testBcR, 100, []BpPeer{{ID: "P1", Height: 120}},
				map[int64]tPBlocks{100: {"P1", true}, 101: {"P1", false}}),
			args:       args{"P1", nil},
			poolWanted: makeBlockPool(testBcR, 100, []BpPeer{}, map[int64]tPBlocks{}),
		},
		{
			name: "delete the shortest of two peers with block requests sent and blocks received",
			pool: makeBlockPool(testBcR, 100, []BpPeer{{ID: "P1", Height: 120}, {ID: "P2", Height: 200}},
				map[int64]tPBlocks{100: {"P1", true}, 101: {"P1", false}}),
			args:       args{"P1", nil},
			poolWanted: makeBlockPool(testBcR, 100, []BpPeer{{ID: "P2", Height: 200}}, map[int64]tPBlocks{}),
		},
		{
			name: "delete the tallest of two peers with block requests sent and blocks received",
			pool: makeBlockPool(testBcR, 100, []BpPeer{{ID: "P1", Height: 120}, {ID: "P2", Height: 110}},
				map[int64]tPBlocks{100: {"P1", true}, 101: {"P1", false}}),
			args:       args{"P1", nil},
			poolWanted: makeBlockPool(testBcR, 100, []BpPeer{{ID: "P2", Height: 110}}, map[int64]tPBlocks{}),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			tt.pool.RemovePeer(tt.args.peerID, tt.args.err)
			assertBlockPoolEquivalent(t, tt.poolWanted, tt.pool)
		})
	}
}

func TestBlockPoolRemoveShortPeers(t *testing.T) {
	testBcR := newTestBcR()

	tests := []struct {
		name       string
		pool       *BlockPool
		poolWanted *BlockPool
	}{
		{
			name: "no short peers",
			pool: makeBlockPool(testBcR, 100,
				[]BpPeer{{ID: "P1", Height: 100}, {ID: "P2", Height: 110}, {ID: "P3", Height: 120}}, map[int64]tPBlocks{}),
			poolWanted: makeBlockPool(testBcR, 100,
				[]BpPeer{{ID: "P1", Height: 100}, {ID: "P2", Height: 110}, {ID: "P3", Height: 120}}, map[int64]tPBlocks{}),
		},

		{
			name: "one short peer",
			pool: makeBlockPool(testBcR, 100,
				[]BpPeer{{ID: "P1", Height: 100}, {ID: "P2", Height: 90}, {ID: "P3", Height: 120}}, map[int64]tPBlocks{}),
			poolWanted: makeBlockPool(testBcR, 100,
				[]BpPeer{{ID: "P1", Height: 100}, {ID: "P3", Height: 120}}, map[int64]tPBlocks{}),
		},

		{
			name: "all short peers",
			pool: makeBlockPool(testBcR, 100,
				[]BpPeer{{ID: "P1", Height: 90}, {ID: "P2", Height: 91}, {ID: "P3", Height: 92}}, map[int64]tPBlocks{}),
			poolWanted: makeBlockPool(testBcR, 100, []BpPeer{}, map[int64]tPBlocks{}),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			pool := tt.pool
			pool.removeShortPeers()
			assertBlockPoolEquivalent(t, tt.poolWanted, tt.pool)
		})
	}
}

func TestBlockPoolSendRequestBatch(t *testing.T) {
	type testPeerResult struct {
		id                      p2p.ID
		numPendingBlockRequests int
	}

	testBcR := newTestBcR()

	tests := []struct {
		name               string
		pool               *BlockPool
		maxRequestsPerPeer int
		expRequests        map[int64]bool
		expRequestsSent    int
		expPeerResults     []testPeerResult
	}{
		{
			name:               "one peer - send up to maxRequestsPerPeer block requests",
			pool:               makeBlockPool(testBcR, 10, []BpPeer{{ID: "P1", Height: 100}}, map[int64]tPBlocks{}),
			maxRequestsPerPeer: 2,
			expRequests:        map[int64]bool{10: true, 11: true},
			expRequestsSent:    2,
			expPeerResults:     []testPeerResult{{id: "P1", numPendingBlockRequests: 2}},
		},
		{
			name: "multiple peers - stops at gap between height and base",
			pool: makeBlockPool(testBcR, 10, []BpPeer{
				{ID: "P1", Base: 1, Height: 12},
				{ID: "P2", Base: 15, Height: 100},
			}, map[int64]tPBlocks{}),
			maxRequestsPerPeer: 10,
			expRequests:        map[int64]bool{10: true, 11: true, 12: true},
			expRequestsSent:    3,
			expPeerResults: []testPeerResult{
				{id: "P1", numPendingBlockRequests: 3},
				{id: "P2", numPendingBlockRequests: 0},
			},
		},
		{
			name: "n peers - send n*maxRequestsPerPeer block requests",
			pool: makeBlockPool(
				testBcR,
				10,
				[]BpPeer{{ID: "P1", Height: 100}, {ID: "P2", Height: 100}},
				map[int64]tPBlocks{}),
			maxRequestsPerPeer: 2,
			expRequests:        map[int64]bool{10: true, 11: true},
			expRequestsSent:    4,
			expPeerResults: []testPeerResult{
				{id: "P1", numPendingBlockRequests: 2},
				{id: "P2", numPendingBlockRequests: 2}},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			resetPoolTestResults()

			var pool = tt.pool
			maxRequestsPerPeer = tt.maxRequestsPerPeer
			pool.MakeNextRequests(10)

			assert.Equal(t, tt.expRequestsSent, testResults.numRequestsSent)
			for _, tPeer := range tt.expPeerResults {
				var peer = pool.peers[tPeer.id]
				assert.NotNil(t, peer)
				assert.Equal(t, tPeer.numPendingBlockRequests, peer.NumPendingBlockRequests)
			}
		})
	}
}

func TestBlockPoolAddBlock(t *testing.T) {
	testBcR := newTestBcR()
	txs := []types.Tx{types.Tx("foo"), types.Tx("bar")}

	coreChainLock := types.NewMockChainLock(1)

	type args struct {
		peerID    p2p.ID
		block     *types.Block
		blockSize int
	}
	tests := []struct {
		name       string
		pool       *BlockPool
		args       args
		poolWanted *BlockPool
		errWanted  error
	}{
		{name: "block from unknown peer",
			pool: makeBlockPool(testBcR, 10, []BpPeer{{ID: "P1", Height: 100}}, map[int64]tPBlocks{}),
			args: args{
				peerID:    "P2",
				block:     types.MakeBlock(int64(10), coreChainLock.CoreBlockHeight, &coreChainLock, txs, nil, nil),
				blockSize: 100,
			},
			poolWanted: makeBlockPool(testBcR, 10, []BpPeer{{ID: "P1", Height: 100}}, map[int64]tPBlocks{}),
			errWanted:  errBadDataFromPeer,
		},
		{name: "unexpected block 11 from known peer - waiting for 10",
			pool: makeBlockPool(testBcR, 10,
				[]BpPeer{{ID: "P1", Height: 100}},
				map[int64]tPBlocks{10: {"P1", false}}),
			args: args{
				peerID:    "P1",
				block:     types.MakeBlock(int64(11), coreChainLock.CoreBlockHeight, &coreChainLock, txs, nil, nil),
				blockSize: 100,
			},
			poolWanted: makeBlockPool(testBcR, 10,
				[]BpPeer{{ID: "P1", Height: 100}},
				map[int64]tPBlocks{10: {"P1", false}}),
			errWanted: errMissingBlock,
		},
		{name: "unexpected block 10 from known peer - already have 10",
			pool: makeBlockPool(testBcR, 10,
				[]BpPeer{{ID: "P1", Height: 100}},
				map[int64]tPBlocks{10: {"P1", true}, 11: {"P1", false}}),
			args: args{
				peerID:    "P1",
				block:     types.MakeBlock(int64(10), coreChainLock.CoreBlockHeight, &coreChainLock, txs, nil, nil),
				blockSize: 100,
			},
			poolWanted: makeBlockPool(testBcR, 10,
				[]BpPeer{{ID: "P1", Height: 100}},
				map[int64]tPBlocks{10: {"P1", true}, 11: {"P1", false}}),
			errWanted: errDuplicateBlock,
		},
		{name: "unexpected block 10 from known peer P2 - expected 10 to come from P1",
			pool: makeBlockPool(testBcR, 10,
				[]BpPeer{{ID: "P1", Height: 100}, {ID: "P2", Height: 100}},
				map[int64]tPBlocks{10: {"P1", false}}),
			args: args{
				peerID:    "P2",
				block:     types.MakeBlock(int64(10), coreChainLock.CoreBlockHeight, &coreChainLock, txs, nil, nil),
				blockSize: 100,
			},
			poolWanted: makeBlockPool(testBcR, 10,
				[]BpPeer{{ID: "P1", Height: 100}, {ID: "P2", Height: 100}},
				map[int64]tPBlocks{10: {"P1", false}}),
			errWanted: errBadDataFromPeer,
		},
		{name: "expected block from known peer",
			pool: makeBlockPool(testBcR, 10,
				[]BpPeer{{ID: "P1", Height: 100}},
				map[int64]tPBlocks{10: {"P1", false}}),
			args: args{
				peerID:    "P1",
				block:     types.MakeBlock(int64(10), coreChainLock.CoreBlockHeight, &coreChainLock, txs, nil, nil),
				blockSize: 100,
			},
			poolWanted: makeBlockPool(testBcR, 10,
				[]BpPeer{{ID: "P1", Height: 100}},
				map[int64]tPBlocks{10: {"P1", true}}),
			errWanted: nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			err := tt.pool.AddBlock(tt.args.peerID, tt.args.block, tt.args.blockSize)
			assert.Equal(t, tt.errWanted, err)
			assertBlockPoolEquivalent(t, tt.poolWanted, tt.pool)
		})
	}
}

func TestBlockPoolFirstTwoBlocksAndPeers(t *testing.T) {
	testBcR := newTestBcR()

	tests := []struct {
		name         string
		pool         *BlockPool
		firstWanted  int64
		secondWanted int64
		errWanted    error
	}{
		{
			name: "both blocks missing",
			pool: makeBlockPool(testBcR, 10,
				[]BpPeer{{ID: "P1", Height: 100}, {ID: "P2", Height: 100}},
				map[int64]tPBlocks{15: {"P1", true}, 16: {"P2", true}}),
			errWanted: errMissingBlock,
		},
		{
			name: "second block missing",
			pool: makeBlockPool(testBcR, 15,
				[]BpPeer{{ID: "P1", Height: 100}, {ID: "P2", Height: 100}},
				map[int64]tPBlocks{15: {"P1", true}, 18: {"P2", true}}),
			firstWanted: 15,
			errWanted:   errMissingBlock,
		},
		{
			name: "first block missing",
			pool: makeBlockPool(testBcR, 15,
				[]BpPeer{{ID: "P1", Height: 100}, {ID: "P2", Height: 100}},
				map[int64]tPBlocks{16: {"P2", true}, 18: {"P2", true}}),
			secondWanted: 16,
			errWanted:    errMissingBlock,
		},
		{
			name: "both blocks present",
			pool: makeBlockPool(testBcR, 10,
				[]BpPeer{{ID: "P1", Height: 100}, {ID: "P2", Height: 100}},
				map[int64]tPBlocks{10: {"P1", true}, 11: {"P2", true}}),
			firstWanted:  10,
			secondWanted: 11,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			pool := tt.pool
			gotFirst, gotSecond, err := pool.FirstTwoBlocksAndPeers()
			assert.Equal(t, tt.errWanted, err)

			if tt.firstWanted != 0 {
				peer := pool.blocks[tt.firstWanted]
				block := pool.peers[peer].blocks[tt.firstWanted]
				assert.Equal(t, block, gotFirst.block,
					"BlockPool.FirstTwoBlocksAndPeers() gotFirst = %v, want %v",
					tt.firstWanted, gotFirst.block.Height)
			}

			if tt.secondWanted != 0 {
				peer := pool.blocks[tt.secondWanted]
				block := pool.peers[peer].blocks[tt.secondWanted]
				assert.Equal(t, block, gotSecond.block,
					"BlockPool.FirstTwoBlocksAndPeers() gotFirst = %v, want %v",
					tt.secondWanted, gotSecond.block.Height)
			}
		})
	}
}

func TestBlockPoolInvalidateFirstTwoBlocks(t *testing.T) {
	testBcR := newTestBcR()

	tests := []struct {
		name       string
		pool       *BlockPool
		poolWanted *BlockPool
	}{
		{
			name: "both blocks missing",
			pool: makeBlockPool(testBcR, 10,
				[]BpPeer{{ID: "P1", Height: 100}, {ID: "P2", Height: 100}},
				map[int64]tPBlocks{15: {"P1", true}, 16: {"P2", true}}),
			poolWanted: makeBlockPool(testBcR, 10,
				[]BpPeer{{ID: "P1", Height: 100}, {ID: "P2", Height: 100}},
				map[int64]tPBlocks{15: {"P1", true}, 16: {"P2", true}}),
		},
		{
			name: "second block missing",
			pool: makeBlockPool(testBcR, 15,
				[]BpPeer{{ID: "P1", Height: 100}, {ID: "P2", Height: 100}},
				map[int64]tPBlocks{15: {"P1", true}, 18: {"P2", true}}),
			poolWanted: makeBlockPool(testBcR, 15,
				[]BpPeer{{ID: "P2", Height: 100}},
				map[int64]tPBlocks{18: {"P2", true}}),
		},
		{
			name: "first block missing",
			pool: makeBlockPool(testBcR, 15,
				[]BpPeer{{ID: "P1", Height: 100}, {ID: "P2", Height: 100}},
				map[int64]tPBlocks{18: {"P1", true}, 16: {"P2", true}}),
			poolWanted: makeBlockPool(testBcR, 15,
				[]BpPeer{{ID: "P1", Height: 100}},
				map[int64]tPBlocks{18: {"P1", true}}),
		},
		{
			name: "both blocks present",
			pool: makeBlockPool(testBcR, 10,
				[]BpPeer{{ID: "P1", Height: 100}, {ID: "P2", Height: 100}},
				map[int64]tPBlocks{10: {"P1", true}, 11: {"P2", true}}),
			poolWanted: makeBlockPool(testBcR, 10,
				[]BpPeer{},
				map[int64]tPBlocks{}),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			tt.pool.InvalidateFirstTwoBlocks(errNoPeerResponse)
			assertBlockPoolEquivalent(t, tt.poolWanted, tt.pool)
		})
	}
}

func TestProcessedCurrentHeightBlock(t *testing.T) {
	testBcR := newTestBcR()

	tests := []struct {
		name       string
		pool       *BlockPool
		poolWanted *BlockPool
	}{
		{
			name: "one peer",
			pool: makeBlockPool(testBcR, 100, []BpPeer{{ID: "P1", Height: 120}},
				map[int64]tPBlocks{100: {"P1", true}, 101: {"P1", true}}),
			poolWanted: makeBlockPool(testBcR, 101, []BpPeer{{ID: "P1", Height: 120}},
				map[int64]tPBlocks{101: {"P1", true}}),
		},
		{
			name: "multiple peers",
			pool: makeBlockPool(testBcR, 100,
				[]BpPeer{{ID: "P1", Height: 120}, {ID: "P2", Height: 120}, {ID: "P3", Height: 130}},
				map[int64]tPBlocks{
					100: {"P1", true}, 104: {"P1", true}, 105: {"P1", false},
					101: {"P2", true}, 103: {"P2", false},
					102: {"P3", true}, 106: {"P3", true}}),
			poolWanted: makeBlockPool(testBcR, 101,
				[]BpPeer{{ID: "P1", Height: 120}, {ID: "P2", Height: 120}, {ID: "P3", Height: 130}},
				map[int64]tPBlocks{
					104: {"P1", true}, 105: {"P1", false},
					101: {"P2", true}, 103: {"P2", false},
					102: {"P3", true}, 106: {"P3", true}}),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			tt.pool.ProcessedCurrentHeightBlock()
			assertBlockPoolEquivalent(t, tt.poolWanted, tt.pool)
		})
	}
}

func TestRemovePeerAtCurrentHeight(t *testing.T) {
	testBcR := newTestBcR()

	tests := []struct {
		name       string
		pool       *BlockPool
		poolWanted *BlockPool
	}{
		{
			name: "one peer, remove peer for block at H",
			pool: makeBlockPool(testBcR, 100, []BpPeer{{ID: "P1", Height: 120}},
				map[int64]tPBlocks{100: {"P1", false}, 101: {"P1", true}}),
			poolWanted: makeBlockPool(testBcR, 100, []BpPeer{}, map[int64]tPBlocks{}),
		},
		{
			name: "one peer, remove peer for block at H+1",
			pool: makeBlockPool(testBcR, 100, []BpPeer{{ID: "P1", Height: 120}},
				map[int64]tPBlocks{100: {"P1", true}, 101: {"P1", false}}),
			poolWanted: makeBlockPool(testBcR, 100, []BpPeer{}, map[int64]tPBlocks{}),
		},
		{
			name: "multiple peers, remove peer for block at H",
			pool: makeBlockPool(testBcR, 100,
				[]BpPeer{{ID: "P1", Height: 120}, {ID: "P2", Height: 120}, {ID: "P3", Height: 130}},
				map[int64]tPBlocks{
					100: {"P1", false}, 104: {"P1", true}, 105: {"P1", false},
					101: {"P2", true}, 103: {"P2", false},
					102: {"P3", true}, 106: {"P3", true}}),
			poolWanted: makeBlockPool(testBcR, 100,
				[]BpPeer{{ID: "P2", Height: 120}, {ID: "P3", Height: 130}},
				map[int64]tPBlocks{
					101: {"P2", true}, 103: {"P2", false},
					102: {"P3", true}, 106: {"P3", true}}),
		},
		{
			name: "multiple peers, remove peer for block at H+1",
			pool: makeBlockPool(testBcR, 100,
				[]BpPeer{{ID: "P1", Height: 120}, {ID: "P2", Height: 120}, {ID: "P3", Height: 130}},
				map[int64]tPBlocks{
					100: {"P1", true}, 104: {"P1", true}, 105: {"P1", false},
					101: {"P2", false}, 103: {"P2", false},
					102: {"P3", true}, 106: {"P3", true}}),
			poolWanted: makeBlockPool(testBcR, 100,
				[]BpPeer{{ID: "P1", Height: 120}, {ID: "P3", Height: 130}},
				map[int64]tPBlocks{
					100: {"P1", true}, 104: {"P1", true}, 105: {"P1", false},
					102: {"P3", true}, 106: {"P3", true}}),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			tt.pool.RemovePeerAtCurrentHeights(errNoPeerResponse)
			assertBlockPoolEquivalent(t, tt.poolWanted, tt.pool)
		})
	}
}
