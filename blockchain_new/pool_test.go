package blockchain_new

import (
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"

	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/types"
)

type fields struct {
	logger        log.Logger
	peers         map[p2p.ID]*bpPeer
	blocks        map[int64]*blockData
	height        int64
	maxPeerHeight int64
}

type testPeer struct {
	id     p2p.ID
	height int64
}

func testErrFunc(err error, peerID p2p.ID) {
}

func makeUpdateFields(log log.Logger, height int64, peers []testPeer, generateBlocks bool) fields {
	ufields := fields{
		logger: log,
		height: height,
		peers:  make(map[p2p.ID]*bpPeer),
		blocks: make(map[int64]*blockData),
	}

	var maxH int64
	for _, p := range peers {
		if p.height > maxH {
			maxH = p.height
		}
		ufields.peers[p.id] = newBPPeer(p.id, p.height, testErrFunc)
	}
	ufields.maxPeerHeight = maxH
	return ufields
}

func poolCopy(pool *blockPool) *blockPool {
	return &blockPool{
		peers:         peersCopy(pool.peers),
		logger:        pool.logger,
		blocks:        blocksCopy(pool.blocks),
		height:        pool.height,
		maxPeerHeight: pool.maxPeerHeight,
	}
}

func blocksCopy(blocks map[int64]*blockData) map[int64]*blockData {
	blockCopy := make(map[int64]*blockData)
	for _, b := range blocks {
		blockCopy[b.block.Height] = &blockData{peerId: b.peerId, block: b.block}

	}
	return blockCopy
}

func peersCopy(peers map[p2p.ID]*bpPeer) map[p2p.ID]*bpPeer {
	peerCopy := make(map[p2p.ID]*bpPeer)
	for _, p := range peers {
		peerCopy[p.id] = newBPPeer(p.id, p.height, p.errFunc)
	}
	return peerCopy
}

func TestBlockPoolUpdatePeer(t *testing.T) {
	l := log.TestingLogger()
	type args struct {
		peerID  p2p.ID
		height  int64
		errFunc func(err error, peerID p2p.ID)
	}

	tests := []struct {
		name            string
		fields          fields
		args            args
		errWanted       error
		addWanted       bool
		delWanted       bool
		maxHeightWanted int64
	}{

		{
			name:            "add a first short peer",
			fields:          makeUpdateFields(l, 100, []testPeer{}, false),
			args:            args{"P1", 50, func(err error, peerId p2p.ID) {}},
			errWanted:       errPeerTooShort,
			maxHeightWanted: int64(0),
		},
		{
			name:            "add a first good peer",
			fields:          makeUpdateFields(l, 100, []testPeer{}, false),
			args:            args{"P1", 101, func(err error, peerId p2p.ID) {}},
			addWanted:       true,
			maxHeightWanted: int64(101),
		},
		{
			name:            "increase the height of P1 from 120 to 123",
			fields:          makeUpdateFields(l, 100, []testPeer{{"P1", 120}}, false),
			args:            args{"P1", 123, func(err error, peerId p2p.ID) {}},
			maxHeightWanted: int64(123),
		},
		{
			name:            "decrease the height of P1 from 120 to 110",
			fields:          makeUpdateFields(l, 100, []testPeer{{"P1", 120}}, false),
			args:            args{"P1", 110, func(err error, peerId p2p.ID) {}},
			maxHeightWanted: int64(110),
		},
		{
			name:            "decrease the height of P1 from 120 to 90",
			fields:          makeUpdateFields(l, 100, []testPeer{{"P1", 120}}, false),
			args:            args{"P1", 90, func(err error, peerId p2p.ID) {}},
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
			}

			beforePool := poolCopy(pool)
			err := pool.updatePeer(tt.args.peerID, tt.args.height, tt.args.errFunc)
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
			assert.Equal(t, pool.peers[tt.args.peerID].height, tt.args.height)
			assert.Equal(t, tt.maxHeightWanted, pool.maxPeerHeight)

		})
	}
}

func TestBlockPoolRemovePeer(t *testing.T) {
	type args struct {
		peerID p2p.ID
		err    error
	}

	l := log.TestingLogger()

	tests := []struct {
		name            string
		fields          fields
		args            args
		maxHeightWanted int64
	}{
		{
			name:            "attempt to delete non-existing peer",
			fields:          makeUpdateFields(l, 100, []testPeer{{"P1", 120}}, false),
			args:            args{"P99", nil},
			maxHeightWanted: int64(120),
		},
		{
			name:            "delete the only peer",
			fields:          makeUpdateFields(l, 100, []testPeer{{"P1", 120}}, false),
			args:            args{"P1", nil},
			maxHeightWanted: int64(0),
		},
		{
			name:            "delete shorter of two peers",
			fields:          makeUpdateFields(l, 100, []testPeer{{"P1", 100}, {"P2", 120}}, false),
			args:            args{"P1", nil},
			maxHeightWanted: int64(120),
		},
		{
			name:            "delete taller of two peers",
			fields:          makeUpdateFields(l, 100, []testPeer{{"P1", 100}, {"P2", 120}}, false),
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

func TestBlockPoolRemoveShortPeers(t *testing.T) {

	l := log.TestingLogger()

	tests := []struct {
		name            string
		fields          fields
		maxHeightWanted int64
		noChange        bool
	}{
		{
			name: "no short peers",
			fields: makeUpdateFields(l, 100,
				[]testPeer{{"P1", 100}, {"P2", 110}, {"P3", 120}},
				false),
			maxHeightWanted: int64(120),
			noChange:        true,
		},
		{
			name: "one short peers",
			fields: makeUpdateFields(l, 100,
				[]testPeer{{"P1", 100}, {"P2", 90}, {"P3", 120}},
				false),
			maxHeightWanted: int64(120),
		},
		{
			name: "all short peers",
			fields: makeUpdateFields(l, 100,
				[]testPeer{{"P1", 90}, {"P2", 91}, {"P3", 92}},
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

func TestBlockPoolSendRequestBatch(t *testing.T) {

	type args struct {
		sendFunc func(peerID p2p.ID, height int64) error
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
			if err := pool.sendRequestBatch(tt.args.sendFunc); (err != nil) != tt.wantErr {
				t.Errorf("blockPool.sendRequestBatch() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBlockPoolGetBestPeer(t *testing.T) {

	type args struct {
		height int64
	}
	tests := []struct {
		name       string
		fields     fields
		args       args
		wantPeerId p2p.ID
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
			gotPeerId, err := pool.getBestPeer(tt.args.height)
			if (err != nil) != tt.wantErr {
				t.Errorf("blockPool.getBestPeer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotPeerId, tt.wantPeerId) {
				t.Errorf("blockPool.getBestPeer() = %v, want %v", gotPeerId, tt.wantPeerId)
			}
		})
	}
}
