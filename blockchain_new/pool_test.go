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

type fields struct {
	logger        log.Logger
	peers         map[p2p.ID]*bpPeer
	blocks        map[int64]p2p.ID
	height        int64
	maxPeerHeight int64
}

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

func makeUpdateFields(bcr *testBcR, height int64, peers []bpPeer, generateBlocks bool) fields {
	uFields := fields{
		logger: bcr.logger,
		height: height,
		peers:  make(map[p2p.ID]*bpPeer),
		blocks: make(map[int64]p2p.ID),
	}

	var maxH int64
	for _, p := range peers {
		if p.height > maxH {
			maxH = p.height
		}
		uFields.peers[p.id] = newBPPeer(p.id, p.height, bcr.sendPeerError)
		uFields.peers[p.id].setLogger(bcr.logger)

	}
	uFields.maxPeerHeight = maxH
	return uFields
}

func poolCopy(pool *blockPool) *blockPool {
	return &blockPool{
		peers:         peersCopy(pool.peers),
		logger:        pool.logger,
		blocks:        pool.blocks,
		height:        pool.height,
		maxPeerHeight: pool.maxPeerHeight,
		toBcR:         pool.toBcR,
	}
}

func peersCopy(peers map[p2p.ID]*bpPeer) map[p2p.ID]*bpPeer {
	peerCopy := make(map[p2p.ID]*bpPeer)
	for _, p := range peers {
		peerCopy[p.id] = newBPPeer(p.id, p.height, p.errFunc)
	}
	return peerCopy
}

func TestBlockPoolUpdateEmptyPeer(t *testing.T) {
	testBcR := newTestBcR()

	tests := []struct {
		name            string
		fields          fields
		args            testPeer
		errWanted       error
		addWanted       bool
		delWanted       bool
		maxHeightWanted int64
	}{

		{
			name:            "add a first short peer",
			fields:          makeUpdateFields(testBcR, 100, []bpPeer{}, false),
			args:            testPeer{"P1", 50},
			errWanted:       errPeerTooShort,
			maxHeightWanted: int64(0),
		},
		{
			name:            "add a first good peer",
			fields:          makeUpdateFields(testBcR, 100, []bpPeer{}, false),
			args:            testPeer{"P1", 101},
			addWanted:       true,
			maxHeightWanted: int64(101),
		},
		{
			name:            "increase the height of P1 from 120 to 123",
			fields:          makeUpdateFields(testBcR, 100, []bpPeer{{id: "P1", height: 120}}, false),
			args:            testPeer{"P1", 123},
			maxHeightWanted: int64(123),
		},
		{
			name:            "decrease the height of P1 from 120 to 110",
			fields:          makeUpdateFields(testBcR, 100, []bpPeer{{id: "P1", height: 120}}, false),
			args:            testPeer{"P1", 110},
			maxHeightWanted: int64(110),
		},
		{
			name:            "decrease the height of P1 from 120 to 90",
			fields:          makeUpdateFields(testBcR, 100, []bpPeer{{id: "P1", height: 120}}, false),
			args:            testPeer{"P1", 90},
			delWanted:       true,
			errWanted:       errPeerTooShort,
			maxHeightWanted: int64(0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &blockPool{
				logger:        tt.fields.logger,
				peers:         tt.fields.peers,
				blocks:        tt.fields.blocks,
				height:        tt.fields.height,
				maxPeerHeight: tt.fields.maxPeerHeight,
				toBcR:         testBcR,
			}

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

func TestBlockPoolRemoveEmptyPeer(t *testing.T) {
	testBcR := newTestBcR()

	type args struct {
		peerID p2p.ID
		err    error
	}

	tests := []struct {
		name            string
		fields          fields
		args            args
		maxHeightWanted int64
	}{
		{
			name:            "attempt to delete non-existing peer",
			fields:          makeUpdateFields(testBcR, 100, []bpPeer{{id: "P1", height: 120}}, false),
			args:            args{"P99", nil},
			maxHeightWanted: int64(120),
		},
		{
			name:            "delete the only peer",
			fields:          makeUpdateFields(testBcR, 100, []bpPeer{{id: "P1", height: 120}}, false),
			args:            args{"P1", nil},
			maxHeightWanted: int64(0),
		},
		{
			name:            "delete the shortest of two peers",
			fields:          makeUpdateFields(testBcR, 100, []bpPeer{{id: "P1", height: 100}, {id: "P2", height: 120}}, false),
			args:            args{"P1", nil},
			maxHeightWanted: int64(120),
		},
		{
			name:            "delete the tallest of two peers",
			fields:          makeUpdateFields(testBcR, 100, []bpPeer{{id: "P1", height: 100}, {id: "P2", height: 120}}, false),
			args:            args{"P2", nil},
			maxHeightWanted: int64(100),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &blockPool{
				logger:        tt.fields.logger,
				peers:         tt.fields.peers,
				blocks:        tt.fields.blocks,
				height:        tt.fields.height,
				maxPeerHeight: tt.fields.maxPeerHeight,
			}
			pool.removePeer(tt.args.peerID, tt.args.err)
			assert.Equal(t, tt.maxHeightWanted, pool.maxPeerHeight)
			_, ok := pool.peers[tt.args.peerID]
			assert.False(t, ok)
		})
	}
}

func TestBlockPoolRemoveShortEmptyPeers(t *testing.T) {
	testBcR := newTestBcR()

	tests := []struct {
		name            string
		fields          fields
		maxHeightWanted int64
		noChange        bool
	}{
		{
			name: "no short peers",
			fields: makeUpdateFields(testBcR, 100,
				[]bpPeer{{id: "P1", height: 100}, {id: "P2", height: 110}, {id: "P3", height: 120}},
				false),
			maxHeightWanted: int64(120),
			noChange:        true,
		},
		{
			name: "one short peers",
			fields: makeUpdateFields(testBcR, 100,
				[]bpPeer{{id: "P1", height: 100}, {id: "P2", height: 90}, {id: "P3", height: 120}},
				false),
			maxHeightWanted: int64(120),
		},
		{
			name: "all short peers",
			fields: makeUpdateFields(testBcR, 100,
				[]bpPeer{{id: "P1", height: 90}, {id: "P2", height: 91}, {id: "P3", height: 92}},
				false),
			maxHeightWanted: int64(0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &blockPool{
				logger:        tt.fields.logger,
				peers:         tt.fields.peers,
				blocks:        tt.fields.blocks,
				height:        tt.fields.height,
				maxPeerHeight: tt.fields.maxPeerHeight,
			}

			beforePool := poolCopy(pool)

			pool.removeShortPeers()
			assert.Equal(t, tt.maxHeightWanted, pool.maxPeerHeight)
			if tt.noChange {
				assert.Equal(t, len(beforePool.peers), len(pool.peers))
				return
			}
			for _, peer := range tt.fields.peers {
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
		fields             fields
		maxRequestsPerPeer int32
		expRequests        map[int64]bool
		expPeerResults     []testPeerResult
		expNumPending      int32
	}{
		{
			name:               "one peer - send up to maxRequestsPerPeer block requests",
			fields:             makeUpdateFields(testBcR, 10, []bpPeer{{id: "P1", height: 100}}, false),
			maxRequestsPerPeer: 2,
			expRequests:        map[int64]bool{10: true, 11: true},
			expPeerResults:     []testPeerResult{{id: "P1", height: 100, numPending: 2, blocks: map[int64]*types.Block{10: nil, 11: nil}}},
			expNumPending:      2,
		},
		{
			name:               "n peers - send n*maxRequestsPerPeer block requests",
			fields:             makeUpdateFields(testBcR, 10, []bpPeer{{id: "P1", height: 100}, {id: "P2", height: 100}}, false),
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

			pool := &blockPool{
				logger:            tt.fields.logger,
				peers:             tt.fields.peers,
				blocks:            tt.fields.blocks,
				requests:          make(map[int64]bool),
				height:            tt.fields.height,
				nextRequestHeight: tt.fields.height,
				maxPeerHeight:     tt.fields.maxPeerHeight,
				toBcR:             testBcR,
			}

			maxRequestsPerPeer = int32(tt.maxRequestsPerPeer)
			//beforePool := poolCopy(pool)
			err := pool.makeNextRequests(10)
			assert.Nil(t, err)
			assert.Equal(t, tt.expNumPending, pool.numPending)
			assert.Equal(t, testResults.numRequestsSent, maxRequestsPerPeer*int32(len(pool.peers)))

			for _, tPeer := range tt.expPeerResults {
				peer := pool.peers[tPeer.id]
				assert.NotNil(t, peer)
				assert.Equal(t, tPeer.numPending, peer.numPending)
				/*
					fmt.Println("tt", tt.name, "peer", peer.id, "expected:", tPeer.blocks, "actual:", peer.blocks)
					assert.Equal(t, tPeer.blocks, peer.blocks)
					for h, tBl := range tPeer.blocks {
						block := peer.blocks[h]
						assert.Equal(t, tBl, block)
					}
				*/
			}
			assert.Equal(t, testResults.numRequestsSent, maxRequestsPerPeer*int32(len(pool.peers)))

		})
	}
}

func TestBlockPoolAddBlock(t *testing.T) {
	type args struct {
		peerID    p2p.ID
		block     *types.Block
		blockSize int
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &blockPool{
				logger:        tt.fields.logger,
				peers:         tt.fields.peers,
				blocks:        tt.fields.blocks,
				height:        tt.fields.height,
				maxPeerHeight: tt.fields.maxPeerHeight,
			}
			if err := pool.addBlock(tt.args.peerID, tt.args.block, tt.args.blockSize); (err != nil) != tt.wantErr {
				t.Errorf("blockPool.addBlock() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBlockPoolGetNextTwoBlocks(t *testing.T) {

	tests := []struct {
		name       string
		fields     fields
		wantFirst  *blockData
		wantSecond *blockData
		wantErr    bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &blockPool{
				logger:        tt.fields.logger,
				peers:         tt.fields.peers,
				blocks:        tt.fields.blocks,
				height:        tt.fields.height,
				maxPeerHeight: tt.fields.maxPeerHeight,
			}
			gotFirst, gotSecond, err := pool.getNextTwoBlocks()
			if (err != nil) != tt.wantErr {
				t.Errorf("blockPool.getNextTwoBlocks() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotFirst, tt.wantFirst) {
				t.Errorf("blockPool.getNextTwoBlocks() gotFirst = %v, want %v", gotFirst, tt.wantFirst)
			}
			if !reflect.DeepEqual(gotSecond, tt.wantSecond) {
				t.Errorf("blockPool.getNextTwoBlocks() gotSecond = %v, want %v", gotSecond, tt.wantSecond)
			}
		})
	}
}

func TestBlockPoolInvalidateFirstTwoBlocks(t *testing.T) {

	type args struct {
		err error
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &blockPool{
				logger:        tt.fields.logger,
				peers:         tt.fields.peers,
				blocks:        tt.fields.blocks,
				height:        tt.fields.height,
				maxPeerHeight: tt.fields.maxPeerHeight,
			}
			pool.invalidateFirstTwoBlocks(tt.args.err)
		})
	}
}

func TestBlockPoolProcessedCurrentHeightBlock(t *testing.T) {

	tests := []struct {
		name   string
		fields fields
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &blockPool{
				logger:        tt.fields.logger,
				peers:         tt.fields.peers,
				blocks:        tt.fields.blocks,
				height:        tt.fields.height,
				maxPeerHeight: tt.fields.maxPeerHeight,
			}
			pool.processedCurrentHeightBlock()
		})
	}
}
