package blockchain

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
	height int64
}

type testBcR struct {
	logger log.Logger
}

type testValues struct {
	numRequestsSent int32
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
func makeBlockPool(bcr *testBcR, height int64, peers []bpPeer, blocks map[int64]tPBlocks) *blockPool {
	bPool := NewBlockPool(height, bcr)
	bPool.SetLogger(bcr.logger)

	txs := []types.Tx{types.Tx("foo"), types.Tx("bar")}

	var maxH int64
	for _, p := range peers {
		if p.height > maxH {
			maxH = p.height
		}
		bPool.peers[p.ID()] = NewBPPeer(p.ID(), p.height, bcr.sendPeerError, nil)
		bPool.peers[p.ID()].SetLogger(bcr.logger)

	}
	bPool.maxPeerHeight = maxH
	for h, p := range blocks {
		bPool.blocks[h] = p.id
		bPool.peers[p.id].blocks[h] = nil
		if p.create {
			// simulate that a block at height h has been received
			bPool.peers[p.id].blocks[h] = types.MakeBlock(int64(h), txs, nil, nil)
		} else {
			// simulate that a request for block at height h has been sent to peer p.id
			bPool.peers[p.id].IncrPending()
		}
	}
	return bPool
}

func assertPeerSetsEquivalent(t *testing.T, set1 map[p2p.ID]*bpPeer, set2 map[p2p.ID]*bpPeer) {
	assert.Equal(t, len(set1), len(set2))
	for peerID, peer1 := range set1 {
		peer2 := set2[peerID]
		assert.NotNil(t, peer2)
		assert.Equal(t, peer1.GetNumPendingBlockRequests(), peer2.GetNumPendingBlockRequests())
		assert.Equal(t, peer1.height, peer2.height)
		assert.Equal(t, len(peer1.blocks), len(peer2.blocks))
		for h, block1 := range peer1.blocks {
			block2 := peer2.blocks[h]
			// block1 and block2 could be nil if a request was made but no block was received
			assert.Equal(t, block1, block2)
		}
	}
}

func assertBlockPoolEquivalent(t *testing.T, poolWanted, pool *blockPool) {
	assert.Equal(t, poolWanted.blocks, pool.blocks)
	assertPeerSetsEquivalent(t, poolWanted.peers, pool.peers)
	assert.Equal(t, poolWanted.maxPeerHeight, pool.maxPeerHeight)
	assert.Equal(t, poolWanted.height, pool.height)

}

func TestBlockPoolUpdatePeer(t *testing.T) {
	testBcR := newTestBcR()

	tests := []struct {
		name       string
		pool       *blockPool
		args       testPeer
		poolWanted *blockPool
		errWanted  error
	}{
		{
			name:       "add a first short peer",
			pool:       makeBlockPool(testBcR, 100, []bpPeer{}, map[int64]tPBlocks{}),
			args:       testPeer{"P1", 50},
			errWanted:  errPeerTooShort,
			poolWanted: makeBlockPool(testBcR, 100, []bpPeer{}, map[int64]tPBlocks{}),
		},
		{
			name:       "add a first good peer",
			pool:       makeBlockPool(testBcR, 100, []bpPeer{}, map[int64]tPBlocks{}),
			args:       testPeer{"P1", 101},
			poolWanted: makeBlockPool(testBcR, 100, []bpPeer{{id: "P1", height: 101}}, map[int64]tPBlocks{}),
		},
		{
			name:       "increase the height of P1 from 120 to 123",
			pool:       makeBlockPool(testBcR, 100, []bpPeer{{id: "P1", height: 120}}, map[int64]tPBlocks{}),
			args:       testPeer{"P1", 123},
			poolWanted: makeBlockPool(testBcR, 100, []bpPeer{{id: "P1", height: 123}}, map[int64]tPBlocks{}),
		},
		{
			name:       "decrease the height of P1 from 120 to 110",
			pool:       makeBlockPool(testBcR, 100, []bpPeer{{id: "P1", height: 120}}, map[int64]tPBlocks{}),
			args:       testPeer{"P1", 110},
			errWanted:  errPeerLowersItsHeight,
			poolWanted: makeBlockPool(testBcR, 100, []bpPeer{}, map[int64]tPBlocks{}),
		},
		{
			name: "decrease the height of P1 from 105 to 102 with blocks",
			pool: makeBlockPool(testBcR, 100, []bpPeer{{id: "P1", height: 105}},
				map[int64]tPBlocks{
					100: {"P1", true}, 101: {"P1", true}, 102: {"P1", true}}),
			args:      testPeer{"P1", 102},
			errWanted: errPeerLowersItsHeight,
			poolWanted: makeBlockPool(testBcR, 100, []bpPeer{},
				map[int64]tPBlocks{}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := tt.pool
			err := pool.UpdatePeer(tt.args.id, tt.args.height)
			assert.Equal(t, tt.errWanted, err)
			assert.Equal(t, tt.poolWanted.blocks, tt.pool.blocks)
			assertPeerSetsEquivalent(t, tt.poolWanted.peers, tt.pool.peers)
			assert.Equal(t, tt.poolWanted.maxPeerHeight, tt.pool.maxPeerHeight)
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
		pool       *blockPool
		args       args
		poolWanted *blockPool
	}{
		{
			name:       "attempt to delete non-existing peer",
			pool:       makeBlockPool(testBcR, 100, []bpPeer{{id: "P1", height: 120}}, map[int64]tPBlocks{}),
			args:       args{"P99", nil},
			poolWanted: makeBlockPool(testBcR, 100, []bpPeer{{id: "P1", height: 120}}, map[int64]tPBlocks{}),
		},
		{
			name:       "delete the only peer without blocks",
			pool:       makeBlockPool(testBcR, 100, []bpPeer{{id: "P1", height: 120}}, map[int64]tPBlocks{}),
			args:       args{"P1", nil},
			poolWanted: makeBlockPool(testBcR, 100, []bpPeer{}, map[int64]tPBlocks{}),
		},
		{
			name:       "delete the shortest of two peers without blocks",
			pool:       makeBlockPool(testBcR, 100, []bpPeer{{id: "P1", height: 100}, {id: "P2", height: 120}}, map[int64]tPBlocks{}),
			args:       args{"P1", nil},
			poolWanted: makeBlockPool(testBcR, 100, []bpPeer{{id: "P2", height: 120}}, map[int64]tPBlocks{}),
		},
		{
			name:       "delete the tallest of two peers without blocks",
			pool:       makeBlockPool(testBcR, 100, []bpPeer{{id: "P1", height: 100}, {id: "P2", height: 120}}, map[int64]tPBlocks{}),
			args:       args{"P2", nil},
			poolWanted: makeBlockPool(testBcR, 100, []bpPeer{{id: "P1", height: 100}}, map[int64]tPBlocks{}),
		},
		{
			name: "delete the only peer with block requests sent and blocks received",
			pool: makeBlockPool(testBcR, 100, []bpPeer{{id: "P1", height: 120}},
				map[int64]tPBlocks{100: {"P1", true}, 101: {"P1", false}}),
			args:       args{"P1", nil},
			poolWanted: makeBlockPool(testBcR, 100, []bpPeer{}, map[int64]tPBlocks{}),
		},
		{
			name: "delete the shortest of two peers with block requests sent and blocks received",
			pool: makeBlockPool(testBcR, 100, []bpPeer{{id: "P1", height: 120}, {id: "P2", height: 200}},
				map[int64]tPBlocks{100: {"P1", true}, 101: {"P1", false}}),
			args:       args{"P1", nil},
			poolWanted: makeBlockPool(testBcR, 100, []bpPeer{{id: "P2", height: 200}}, map[int64]tPBlocks{}),
		},
		{
			name: "delete the tallest of two peers with block requests sent and blocks received",
			pool: makeBlockPool(testBcR, 100, []bpPeer{{id: "P1", height: 120}, {id: "P2", height: 110}},
				map[int64]tPBlocks{100: {"P1", true}, 101: {"P1", false}}),
			args:       args{"P1", nil},
			poolWanted: makeBlockPool(testBcR, 100, []bpPeer{{id: "P2", height: 110}}, map[int64]tPBlocks{}),
		},
	}

	for _, tt := range tests {
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
		pool       *blockPool
		poolWanted *blockPool
	}{
		{
			name: "no short peers",
			pool: makeBlockPool(testBcR, 100,
				[]bpPeer{{id: "P1", height: 100}, {id: "P2", height: 110}, {id: "P3", height: 120}}, map[int64]tPBlocks{}),
			poolWanted: makeBlockPool(testBcR, 100,
				[]bpPeer{{id: "P1", height: 100}, {id: "P2", height: 110}, {id: "P3", height: 120}}, map[int64]tPBlocks{}),
		},

		{
			name: "one short peer",
			pool: makeBlockPool(testBcR, 100,
				[]bpPeer{{id: "P1", height: 100}, {id: "P2", height: 90}, {id: "P3", height: 120}}, map[int64]tPBlocks{}),
			poolWanted: makeBlockPool(testBcR, 100,
				[]bpPeer{{id: "P1", height: 100}, {id: "P3", height: 120}}, map[int64]tPBlocks{}),
		},

		{
			name: "all short peers",
			pool: makeBlockPool(testBcR, 100,
				[]bpPeer{{id: "P1", height: 90}, {id: "P2", height: 91}, {id: "P3", height: 92}}, map[int64]tPBlocks{}),
			poolWanted: makeBlockPool(testBcR, 100, []bpPeer{}, map[int64]tPBlocks{}),
		},
	}

	for _, tt := range tests {
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
		numPendingBlockRequests int32
	}

	testBcR := newTestBcR()

	tests := []struct {
		name                       string
		pool                       *blockPool
		maxRequestsPerPeer         int32
		expRequests                map[int64]bool
		expPeerResults             []testPeerResult
		expnumPendingBlockRequests int32
	}{
		{
			name:                       "one peer - send up to maxRequestsPerPeer block requests",
			pool:                       makeBlockPool(testBcR, 10, []bpPeer{{id: "P1", height: 100}}, map[int64]tPBlocks{}),
			maxRequestsPerPeer:         2,
			expRequests:                map[int64]bool{10: true, 11: true},
			expPeerResults:             []testPeerResult{{id: "P1", numPendingBlockRequests: 2}},
			expnumPendingBlockRequests: 2,
		},
		{
			name:               "n peers - send n*maxRequestsPerPeer block requests",
			pool:               makeBlockPool(testBcR, 10, []bpPeer{{id: "P1", height: 100}, {id: "P2", height: 100}}, map[int64]tPBlocks{}),
			maxRequestsPerPeer: 2,
			expRequests:        map[int64]bool{10: true, 11: true},
			expPeerResults: []testPeerResult{
				{id: "P1", numPendingBlockRequests: 2},
				{id: "P2", numPendingBlockRequests: 2}},
			expnumPendingBlockRequests: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetPoolTestResults()

			var pool = tt.pool
			maxRequestsPerPeer = int32(tt.maxRequestsPerPeer)
			pool.MakeNextRequests(10)
			assert.Equal(t, testResults.numRequestsSent, maxRequestsPerPeer*int32(len(pool.peers)))

			for _, tPeer := range tt.expPeerResults {
				var peer = pool.peers[tPeer.id]
				assert.NotNil(t, peer)
				assert.Equal(t, tPeer.numPendingBlockRequests, peer.GetNumPendingBlockRequests())
			}
			assert.Equal(t, testResults.numRequestsSent, maxRequestsPerPeer*int32(len(pool.peers)))

		})
	}
}

func TestBlockPoolAddBlock(t *testing.T) {
	testBcR := newTestBcR()
	txs := []types.Tx{types.Tx("foo"), types.Tx("bar")}

	type args struct {
		peerID    p2p.ID
		block     *types.Block
		blockSize int
	}
	tests := []struct {
		name       string
		pool       *blockPool
		args       args
		poolWanted *blockPool
		errWanted  error
	}{
		{name: "block from unknown peer",
			pool: makeBlockPool(testBcR, 10, []bpPeer{{id: "P1", height: 100}}, map[int64]tPBlocks{}),
			args: args{
				peerID:    "P2",
				block:     types.MakeBlock(int64(10), txs, nil, nil),
				blockSize: 100,
			},
			poolWanted: makeBlockPool(testBcR, 10, []bpPeer{{id: "P1", height: 100}}, map[int64]tPBlocks{}),
			errWanted:  errBadDataFromPeer,
		},
		{name: "unexpected block 11 from known peer - waiting for 10",
			pool: makeBlockPool(testBcR, 10,
				[]bpPeer{{id: "P1", height: 100}},
				map[int64]tPBlocks{10: {"P1", false}}),
			args: args{
				peerID:    "P1",
				block:     types.MakeBlock(int64(11), txs, nil, nil),
				blockSize: 100,
			},
			poolWanted: makeBlockPool(testBcR, 10,
				[]bpPeer{{id: "P1", height: 100}},
				map[int64]tPBlocks{10: {"P1", false}}),
			errWanted: errBadDataFromPeer,
		},
		{name: "unexpected block 10 from known peer - already have 10",
			pool: makeBlockPool(testBcR, 10,
				[]bpPeer{{id: "P1", height: 100}},
				map[int64]tPBlocks{10: {"P1", true}}),
			args: args{
				peerID:    "P1",
				block:     types.MakeBlock(int64(10), txs, nil, nil),
				blockSize: 100,
			},
			poolWanted: makeBlockPool(testBcR, 10,
				[]bpPeer{{id: "P1", height: 100}},
				map[int64]tPBlocks{10: {"P1", true}}),
			errWanted: errBadDataFromPeer,
		},
		{name: "unexpected block 10 from known peer P2 - expected 10 to come from P1",
			pool: makeBlockPool(testBcR, 10,
				[]bpPeer{{id: "P1", height: 100}, {id: "P2", height: 100}},
				map[int64]tPBlocks{10: {"P1", false}}),
			args: args{
				peerID:    "P2",
				block:     types.MakeBlock(int64(10), txs, nil, nil),
				blockSize: 100,
			},
			poolWanted: makeBlockPool(testBcR, 10,
				[]bpPeer{{id: "P1", height: 100}, {id: "P2", height: 100}},
				map[int64]tPBlocks{10: {"P1", false}}),
			errWanted: errBadDataFromPeer,
		},
		{name: "expected block from known peer",
			pool: makeBlockPool(testBcR, 10,
				[]bpPeer{{id: "P1", height: 100}},
				map[int64]tPBlocks{10: {"P1", false}}),
			args: args{
				peerID:    "P1",
				block:     types.MakeBlock(int64(10), txs, nil, nil),
				blockSize: 100,
			},
			poolWanted: makeBlockPool(testBcR, 10,
				[]bpPeer{{id: "P1", height: 100}},
				map[int64]tPBlocks{10: {"P1", true}}),
			errWanted: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.pool.AddBlock(tt.args.peerID, tt.args.block, tt.args.blockSize)
			assert.Equal(t, tt.errWanted, err)
			assertBlockPoolEquivalent(t, tt.poolWanted, tt.pool)
		})
	}
}

func TestBlockPoolGetNextTwoBlocks(t *testing.T) {
	testBcR := newTestBcR()

	tests := []struct {
		name         string
		pool         *blockPool
		firstWanted  int64
		secondWanted int64
		errWanted    error
	}{
		{
			name: "both blocks missing",
			pool: makeBlockPool(testBcR, 10,
				[]bpPeer{{id: "P1", height: 100}, {id: "P2", height: 100}},
				map[int64]tPBlocks{15: {"P1", true}, 16: {"P2", true}}),
			errWanted: errMissingBlock,
		},
		{
			name: "second block missing",
			pool: makeBlockPool(testBcR, 15,
				[]bpPeer{{id: "P1", height: 100}, {id: "P2", height: 100}},
				map[int64]tPBlocks{15: {"P1", true}, 18: {"P2", true}}),
			firstWanted: 15,
			errWanted:   errMissingBlock,
		},
		{
			name: "first block missing",
			pool: makeBlockPool(testBcR, 15,
				[]bpPeer{{id: "P1", height: 100}, {id: "P2", height: 100}},
				map[int64]tPBlocks{16: {"P2", true}, 18: {"P2", true}}),
			secondWanted: 16,
			errWanted:    errMissingBlock,
		},
		{
			name: "both blocks present",
			pool: makeBlockPool(testBcR, 10,
				[]bpPeer{{id: "P1", height: 100}, {id: "P2", height: 100}},
				map[int64]tPBlocks{10: {"P1", true}, 11: {"P2", true}}),
			firstWanted:  10,
			secondWanted: 11,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := tt.pool
			gotFirst, gotSecond, err := pool.GetNextTwoBlocks()
			assert.Equal(t, tt.errWanted, err)

			if tt.firstWanted != 0 {
				peer := pool.blocks[tt.firstWanted]
				block := pool.peers[peer].blocks[tt.firstWanted]
				assert.Equal(t, gotFirst.block, block,
					"blockPool.getNextTwoBlocks() gotFirst = %v, want %v", gotFirst.block.Height, tt.firstWanted)
			}

			if tt.secondWanted != 0 {
				peer := pool.blocks[tt.secondWanted]
				block := pool.peers[peer].blocks[tt.secondWanted]
				assert.Equal(t, gotSecond.block, block,
					"blockPool.getNextTwoBlocks() gotFirst = %v, want %v", gotSecond.block.Height, tt.secondWanted)
			}
		})
	}
}

func TestBlockPoolInvalidateFirstTwoBlocks(t *testing.T) {
	testBcR := newTestBcR()

	tests := []struct {
		name       string
		pool       *blockPool
		poolWanted *blockPool
	}{
		{
			name: "both blocks missing",
			pool: makeBlockPool(testBcR, 10,
				[]bpPeer{{id: "P1", height: 100}, {id: "P2", height: 100}},
				map[int64]tPBlocks{15: {"P1", true}, 16: {"P2", true}}),
			poolWanted: makeBlockPool(testBcR, 10,
				[]bpPeer{{id: "P1", height: 100}, {id: "P2", height: 100}},
				map[int64]tPBlocks{15: {"P1", true}, 16: {"P2", true}}),
		},
		{
			name: "second block missing",
			pool: makeBlockPool(testBcR, 15,
				[]bpPeer{{id: "P1", height: 100}, {id: "P2", height: 100}},
				map[int64]tPBlocks{15: {"P1", true}, 18: {"P2", true}}),
			poolWanted: makeBlockPool(testBcR, 15,
				[]bpPeer{{id: "P2", height: 100}},
				map[int64]tPBlocks{18: {"P2", true}}),
		},
		{
			name: "first block missing",
			pool: makeBlockPool(testBcR, 15,
				[]bpPeer{{id: "P1", height: 100}, {id: "P2", height: 100}},
				map[int64]tPBlocks{18: {"P1", true}, 16: {"P2", true}}),
			poolWanted: makeBlockPool(testBcR, 15,
				[]bpPeer{{id: "P1", height: 100}},
				map[int64]tPBlocks{18: {"P1", true}}),
		},
		{
			name: "both blocks present",
			pool: makeBlockPool(testBcR, 10,
				[]bpPeer{{id: "P1", height: 100}, {id: "P2", height: 100}},
				map[int64]tPBlocks{10: {"P1", true}, 11: {"P2", true}}),
			poolWanted: makeBlockPool(testBcR, 10,
				[]bpPeer{},
				map[int64]tPBlocks{}),
		},
	}

	for _, tt := range tests {
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
		pool       *blockPool
		poolWanted *blockPool
	}{
		{
			name: "one peer",
			pool: makeBlockPool(testBcR, 100, []bpPeer{{id: "P1", height: 120}},
				map[int64]tPBlocks{100: {"P1", true}, 101: {"P1", true}}),
			poolWanted: makeBlockPool(testBcR, 101, []bpPeer{{id: "P1", height: 120}},
				map[int64]tPBlocks{101: {"P1", true}}),
		},
		{
			name: "multiple peers",
			pool: makeBlockPool(testBcR, 100,
				[]bpPeer{{id: "P1", height: 120}, {id: "P2", height: 120}, {id: "P3", height: 130}},
				map[int64]tPBlocks{
					100: {"P1", true}, 104: {"P1", true}, 105: {"P1", false},
					101: {"P2", true}, 103: {"P2", false},
					102: {"P3", true}, 106: {"P3", true}}),
			poolWanted: makeBlockPool(testBcR, 101,
				[]bpPeer{{id: "P1", height: 120}, {id: "P2", height: 120}, {id: "P3", height: 130}},
				map[int64]tPBlocks{
					104: {"P1", true}, 105: {"P1", false},
					101: {"P2", true}, 103: {"P2", false},
					102: {"P3", true}, 106: {"P3", true}}),
		},
	}

	for _, tt := range tests {
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
		pool       *blockPool
		poolWanted *blockPool
	}{
		{
			name: "one peer, remove peer for block at H",
			pool: makeBlockPool(testBcR, 100, []bpPeer{{id: "P1", height: 120}},
				map[int64]tPBlocks{100: {"P1", false}, 101: {"P1", true}}),
			poolWanted: makeBlockPool(testBcR, 100, []bpPeer{}, map[int64]tPBlocks{}),
		},
		{
			name: "one peer, remove peer for block at H+1",
			pool: makeBlockPool(testBcR, 100, []bpPeer{{id: "P1", height: 120}},
				map[int64]tPBlocks{100: {"P1", true}, 101: {"P1", false}}),
			poolWanted: makeBlockPool(testBcR, 100, []bpPeer{}, map[int64]tPBlocks{}),
		},
		{
			name: "multiple peers, remove peer for block at H",
			pool: makeBlockPool(testBcR, 100,
				[]bpPeer{{id: "P1", height: 120}, {id: "P2", height: 120}, {id: "P3", height: 130}},
				map[int64]tPBlocks{
					100: {"P1", false}, 104: {"P1", true}, 105: {"P1", false},
					101: {"P2", true}, 103: {"P2", false},
					102: {"P3", true}, 106: {"P3", true}}),
			poolWanted: makeBlockPool(testBcR, 100,
				[]bpPeer{{id: "P2", height: 120}, {id: "P3", height: 130}},
				map[int64]tPBlocks{
					101: {"P2", true}, 103: {"P2", false},
					102: {"P3", true}, 106: {"P3", true}}),
		},
		{
			name: "multiple peers, remove peer for block at H+1",
			pool: makeBlockPool(testBcR, 100,
				[]bpPeer{{id: "P1", height: 120}, {id: "P2", height: 120}, {id: "P3", height: 130}},
				map[int64]tPBlocks{
					100: {"P1", true}, 104: {"P1", true}, 105: {"P1", false},
					101: {"P2", false}, 103: {"P2", false},
					102: {"P3", true}, 106: {"P3", true}}),
			poolWanted: makeBlockPool(testBcR, 100,
				[]bpPeer{{id: "P1", height: 120}, {id: "P3", height: 130}},
				map[int64]tPBlocks{
					100: {"P1", true}, 104: {"P1", true}, 105: {"P1", false},
					102: {"P3", true}, 106: {"P3", true}}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.pool.RemovePeerAtCurrentHeights(errNoPeerResponse)
			assertBlockPoolEquivalent(t, tt.poolWanted, tt.pool)
		})
	}
}
