package blockchain_new

import (
	"reflect"
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

type testPeerResult struct {
	id         p2p.ID
	height     int64
	numPending int32
	blocks     map[int64]*types.Block
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

func (testR *testBcR) resetStateTimer(name string, timer *time.Timer, timeout time.Duration, f func()) {
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

func makeBlockPool(bcr *testBcR, height int64, peers []bpPeer, blocks map[int64]tPBlocks) *blockPool {
	bPool := newBlockPool(height, bcr)
	txs := []types.Tx{types.Tx("foo"), types.Tx("bar")}

	var maxH int64
	for _, p := range peers {
		if p.height > maxH {
			maxH = p.height
		}
		bPool.peers[p.id] = newBPPeer(p.id, p.height, bcr.sendPeerError)
		bPool.peers[p.id].setLogger(bcr.logger)

	}
	bPool.maxPeerHeight = maxH
	for h, p := range blocks {
		bPool.blocks[h] = p.id
		bPool.peers[p.id].blocks[h] = nil
		if p.create {
			bPool.peers[p.id].blocks[h] = types.MakeBlock(int64(h), txs, nil, nil)
		} else {
			bPool.peers[p.id].incrPending()
		}
	}
	bPool.setLogger(bcr.logger)
	return bPool
}

func poolCopy(pool *blockPool) *blockPool {
	return &blockPool{
		peers:             peersCopy(pool.peers),
		logger:            pool.logger,
		blocks:            pool.blocks,
		requests:          pool.requests,
		height:            pool.height,
		nextRequestHeight: pool.height,
		maxPeerHeight:     pool.maxPeerHeight,
		toBcR:             pool.toBcR,
	}
}

func peersCopy(peers map[p2p.ID]*bpPeer) map[p2p.ID]*bpPeer {
	peerCopy := make(map[p2p.ID]*bpPeer)
	for _, p := range peers {
		peerCopy[p.id] = newBPPeer(p.id, p.height, p.errFunc)
	}
	return peerCopy
}

func TestBlockPoolUpdatePeerNoBlocks(t *testing.T) {
	testBcR := newTestBcR()

	tests := []struct {
		name            string
		pool            *blockPool
		args            testPeer
		errWanted       error
		addWanted       bool
		delWanted       bool
		maxHeightWanted int64
	}{

		{
			name:            "add a first short peer",
			pool:            makeBlockPool(testBcR, 100, []bpPeer{}, map[int64]tPBlocks{}),
			args:            testPeer{"P1", 50},
			errWanted:       errPeerTooShort,
			maxHeightWanted: int64(0),
		},
		{
			name:            "add a first good peer",
			pool:            makeBlockPool(testBcR, 100, []bpPeer{}, map[int64]tPBlocks{}),
			args:            testPeer{"P1", 101},
			addWanted:       true,
			maxHeightWanted: int64(101),
		},
		{
			name:            "increase the height of P1 from 120 to 123",
			pool:            makeBlockPool(testBcR, 100, []bpPeer{{id: "P1", height: 120}}, map[int64]tPBlocks{}),
			args:            testPeer{"P1", 123},
			maxHeightWanted: int64(123),
		},
		{
			name:            "decrease the height of P1 from 120 to 110",
			pool:            makeBlockPool(testBcR, 100, []bpPeer{{id: "P1", height: 120}}, map[int64]tPBlocks{}),
			args:            testPeer{"P1", 110},
			maxHeightWanted: int64(110),
		},
		{
			name:            "decrease the height of P1 from 120 to 90",
			pool:            makeBlockPool(testBcR, 100, []bpPeer{{id: "P1", height: 120}}, map[int64]tPBlocks{}),
			args:            testPeer{"P1", 90},
			delWanted:       true,
			errWanted:       errPeerTooShort,
			maxHeightWanted: int64(0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := tt.pool

			beforePool := poolCopy(pool)
			err := pool.updatePeer(tt.args.id, tt.args.height)
			if err != tt.errWanted {
				t.Errorf("blockPool.updatePeer() error = %v, wantErr %v", err, tt.errWanted)
			}

			if tt.errWanted != nil {
				// error case
				if tt.delWanted {
					assert.Equal(t, len(beforePool.peers)-1, len(pool.peers))
					return
				}
				assert.Equal(t, beforePool, pool)
				return
			}

			if tt.addWanted {
				// add case only
				assert.Equal(t, len(beforePool.peers)+1, len(pool.peers))
			} else {
				// update case only
				assert.Equal(t, len(beforePool.peers), len(pool.peers))
			}

			// both add and update
			assert.Equal(t, pool.peers[tt.args.id].height, tt.args.height)
			assert.Equal(t, tt.maxHeightWanted, pool.maxPeerHeight)

		})
	}
}

func TestBlockPoolRemovePeerNoBlocks(t *testing.T) {
	testBcR := newTestBcR()

	type args struct {
		peerID p2p.ID
		err    error
	}

	tests := []struct {
		name            string
		pool            *blockPool
		args            args
		maxHeightWanted int64
	}{
		{
			name:            "attempt to delete non-existing peer",
			pool:            makeBlockPool(testBcR, 100, []bpPeer{{id: "P1", height: 120}}, map[int64]tPBlocks{}),
			args:            args{"P99", nil},
			maxHeightWanted: int64(120),
		},
		{
			name:            "delete the only peer",
			pool:            makeBlockPool(testBcR, 100, []bpPeer{{id: "P1", height: 120}}, map[int64]tPBlocks{}),
			args:            args{"P1", nil},
			maxHeightWanted: int64(0),
		},
		{
			name:            "delete the shortest of two peers",
			pool:            makeBlockPool(testBcR, 100, []bpPeer{{id: "P1", height: 100}, {id: "P2", height: 120}}, map[int64]tPBlocks{}),
			args:            args{"P1", nil},
			maxHeightWanted: int64(120),
		},
		{
			name:            "delete the tallest of two peers",
			pool:            makeBlockPool(testBcR, 100, []bpPeer{{id: "P1", height: 100}, {id: "P2", height: 120}}, map[int64]tPBlocks{}),
			args:            args{"P2", nil},
			maxHeightWanted: int64(100),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.pool.removePeer(tt.args.peerID, tt.args.err)
			assert.Equal(t, tt.maxHeightWanted, tt.pool.maxPeerHeight)
			_, ok := tt.pool.peers[tt.args.peerID]
			assert.False(t, ok)
		})
	}
}

func TestBlockPoolRemoveShortPeersNoBlocks(t *testing.T) {
	testBcR := newTestBcR()

	tests := []struct {
		name            string
		pool            *blockPool
		maxHeightWanted int64
		noChange        bool
	}{
		{
			name: "no short peers",
			pool: makeBlockPool(testBcR, 100,
				[]bpPeer{{id: "P1", height: 100}, {id: "P2", height: 110}, {id: "P3", height: 120}},
				map[int64]tPBlocks{}),
			maxHeightWanted: int64(120),
			noChange:        true,
		},
		{
			name: "one short peers",
			pool: makeBlockPool(testBcR, 100,
				[]bpPeer{{id: "P1", height: 100}, {id: "P2", height: 90}, {id: "P3", height: 120}},
				map[int64]tPBlocks{}),
			maxHeightWanted: int64(120),
		},
		{
			name: "all short peers",
			pool: makeBlockPool(testBcR, 100,
				[]bpPeer{{id: "P1", height: 90}, {id: "P2", height: 91}, {id: "P3", height: 92}},
				map[int64]tPBlocks{}),
			maxHeightWanted: int64(0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := tt.pool
			beforePool := poolCopy(pool)

			pool.removeShortPeers()
			assert.Equal(t, tt.maxHeightWanted, pool.maxPeerHeight)
			if tt.noChange {
				assert.Equal(t, len(beforePool.peers), len(pool.peers))
				return
			}
			for _, peer := range tt.pool.peers {
				bPeer, bok := beforePool.peers[peer.id]
				if bok && bPeer.height < beforePool.height {
					_, ok := pool.peers[peer.id]
					assert.False(t, ok)
				}
			}
		})
	}
}

func TestBlockPoolSendRequestBatch(t *testing.T) {
	testBcR := newTestBcR()
	tests := []struct {
		name               string
		pool               *blockPool
		maxRequestsPerPeer int32
		expRequests        map[int64]bool
		expPeerResults     []testPeerResult
		expNumPending      int32
	}{
		{
			name:               "one peer - send up to maxRequestsPerPeer block requests",
			pool:               makeBlockPool(testBcR, 10, []bpPeer{{id: "P1", height: 100}}, map[int64]tPBlocks{}),
			maxRequestsPerPeer: 2,
			expRequests:        map[int64]bool{10: true, 11: true},
			expPeerResults:     []testPeerResult{{id: "P1", height: 100, numPending: 2, blocks: map[int64]*types.Block{10: nil, 11: nil}}},
			expNumPending:      2,
		},
		{
			name:               "n peers - send n*maxRequestsPerPeer block requests",
			pool:               makeBlockPool(testBcR, 10, []bpPeer{{id: "P1", height: 100}, {id: "P2", height: 100}}, map[int64]tPBlocks{}),
			maxRequestsPerPeer: 2,
			expRequests:        map[int64]bool{10: true, 11: true},
			expPeerResults: []testPeerResult{
				{id: "P1", height: 100, numPending: 2, blocks: map[int64]*types.Block{10: nil, 11: nil}},
				{id: "P2", height: 100, numPending: 2, blocks: map[int64]*types.Block{12: nil, 13: nil}}},
			expNumPending: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetPoolTestResults()

			pool := tt.pool

			maxRequestsPerPeer = int32(tt.maxRequestsPerPeer)
			err := pool.makeNextRequests(10)
			assert.Nil(t, err)
			assert.Equal(t, tt.expNumPending, pool.numPending)
			assert.Equal(t, testResults.numRequestsSent, maxRequestsPerPeer*int32(len(pool.peers)))

			for _, tPeer := range tt.expPeerResults {
				peer := pool.peers[tPeer.id]
				assert.NotNil(t, peer)
				assert.Equal(t, tPeer.numPending, peer.numPending)
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
		name    string
		pool    *blockPool
		args    args
		wantErr bool
	}{
		{name: "block from unknown peer",
			pool: makeBlockPool(testBcR, 10, []bpPeer{{id: "P1", height: 100}}, map[int64]tPBlocks{}),
			args: args{
				peerID:    "P2",
				block:     types.MakeBlock(int64(10), txs, nil, nil),
				blockSize: 100,
			},
			wantErr: true,
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
			wantErr: true,
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
			wantErr: true,
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
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := tt.pool
			if err := pool.addBlock(tt.args.peerID, tt.args.block, tt.args.blockSize); (err != nil) != tt.wantErr {
				t.Errorf("blockPool.addBlock() error = %v, wantErr %v", err, tt.wantErr)
			}
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
		wantErr      bool
	}{
		{
			name: "both blocks missing",
			pool: makeBlockPool(testBcR, 10,
				[]bpPeer{{id: "P1", height: 100}, {id: "P2", height: 100}},
				map[int64]tPBlocks{15: {"P1", true}, 16: {"P2", true}}),
			firstWanted:  0,
			secondWanted: 0,
			wantErr:      true,
		},
		{
			name: "second block missing",
			pool: makeBlockPool(testBcR, 15,
				[]bpPeer{{id: "P1", height: 100}, {id: "P2", height: 100}},
				map[int64]tPBlocks{15: {"P1", true}, 18: {"P2", true}}),
			firstWanted:  15,
			secondWanted: 0,
			wantErr:      true,
		},
		{
			name: "first block missing",
			pool: makeBlockPool(testBcR, 15,
				[]bpPeer{{id: "P1", height: 100}, {id: "P2", height: 100}},
				map[int64]tPBlocks{18: {"P1", true}, 16: {"P2", true}}),
			firstWanted:  0,
			secondWanted: 16,
			wantErr:      true,
		},
		{
			name: "both blocks present",
			pool: makeBlockPool(testBcR, 10,
				[]bpPeer{{id: "P1", height: 100}, {id: "P2", height: 100}},
				map[int64]tPBlocks{10: {"P1", true}, 11: {"P2", true}}),
			firstWanted:  10,
			secondWanted: 11,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := tt.pool
			gotFirst, gotSecond, err := pool.getNextTwoBlocks()
			if (err != nil) != tt.wantErr {
				t.Errorf("blockPool.getNextTwoBlocks() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.firstWanted != 0 {
				peer := pool.blocks[tt.firstWanted]
				block := pool.peers[peer].blocks[tt.firstWanted]
				if !reflect.DeepEqual(gotFirst.block, block) {
					t.Errorf("blockPool.getNextTwoBlocks() gotFirst = %v, want %v", gotFirst.block.Height, tt.firstWanted)
				}
			}
			if tt.secondWanted != 0 {
				peer := pool.blocks[tt.secondWanted]
				block := pool.peers[peer].blocks[tt.secondWanted]
				if !reflect.DeepEqual(gotSecond.block, block) {
					t.Errorf("blockPool.getNextTwoBlocks() gotFirst = %v, want %v", gotSecond.block.Height, tt.secondWanted)
				}
			}
		})
	}
}

func TestBlockPoolInvalidateFirstTwoBlocks(t *testing.T) {
	testBcR := newTestBcR()

	tests := []struct {
		name         string
		pool         *blockPool
		firstWanted  int64
		secondWanted int64
		wantChange   bool
	}{
		{
			name: "both blocks missing",
			pool: makeBlockPool(testBcR, 10,
				[]bpPeer{{id: "P1", height: 100}, {id: "P2", height: 100}},
				map[int64]tPBlocks{15: {"P1", true}, 16: {"P2", true}}),
			firstWanted:  0,
			secondWanted: 0,
			wantChange:   false,
		},
		{
			name: "second block missing",
			pool: makeBlockPool(testBcR, 15,
				[]bpPeer{{id: "P1", height: 100}, {id: "P2", height: 100}},
				map[int64]tPBlocks{15: {"P1", true}, 18: {"P2", true}}),
			firstWanted:  15,
			secondWanted: 0,
			wantChange:   true,
		},
		{
			name: "first block missing",
			pool: makeBlockPool(testBcR, 15,
				[]bpPeer{{id: "P1", height: 100}, {id: "P2", height: 100}},
				map[int64]tPBlocks{18: {"P1", true}, 16: {"P2", true}}),
			firstWanted:  0,
			secondWanted: 16,
			wantChange:   true,
		},
		{
			name: "both blocks present",
			pool: makeBlockPool(testBcR, 10,
				[]bpPeer{{id: "P1", height: 100}, {id: "P2", height: 100}},
				map[int64]tPBlocks{10: {"P1", true}, 11: {"P2", true}}),
			firstWanted:  10,
			secondWanted: 11,
			wantChange:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := tt.pool
			gotFirst, gotSecond, _ := pool.getNextTwoBlocks()

			beforePool := poolCopy(pool)
			pool.invalidateFirstTwoBlocks(errNoPeerResponse)
			if !tt.wantChange {
				assert.Equal(t, len(beforePool.peers), len(pool.peers))
				return
			}
			if tt.firstWanted != 0 {
				_, ok := pool.peers[gotFirst.peer.id]
				assert.False(t, ok)
				_, ok = pool.blocks[tt.firstWanted]
				assert.False(t, ok)
				assert.True(t, pool.requests[tt.firstWanted])
			}
			if tt.secondWanted != 0 {
				_, ok := pool.peers[gotSecond.peer.id]
				assert.False(t, ok)
				_, ok = pool.blocks[tt.secondWanted]
				assert.False(t, ok)
				assert.True(t, pool.requests[tt.secondWanted])
			}
		})
	}
}
