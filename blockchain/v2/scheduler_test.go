package v2

import (
	"fmt"
	"math"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/types"
)

type scTestParams struct {
	peers         map[string]*scPeer
	initHeight    int64
	height        int64
	allB          []int64
	pending       map[int64]p2p.ID
	pendingTime   map[int64]time.Time
	received      map[int64]p2p.ID
	peerTimeout   time.Duration
	minRecvRate   int64
	targetPending int
	startTime     time.Time
	syncTimeout   time.Duration
}

func verifyScheduler(sc *scheduler) {
	missing := 0
	if sc.maxHeight() >= sc.height {
		missing = int(math.Min(float64(sc.targetPending), float64(sc.maxHeight()-sc.height+1)))
	}
	if len(sc.blockStates) != missing {
		panic(fmt.Sprintf("scheduler block length %d different than target %d", len(sc.blockStates), missing))
	}
}

func newTestScheduler(params scTestParams) *scheduler {
	peers := make(map[p2p.ID]*scPeer)
	var maxHeight int64

	sc := newScheduler(params.initHeight, params.startTime)
	if params.height != 0 {
		sc.height = params.height
	}

	for id, peer := range params.peers {
		peer.peerID = p2p.ID(id)
		peers[p2p.ID(id)] = peer
		if maxHeight < peer.height {
			maxHeight = peer.height
		}
	}
	for _, h := range params.allB {
		sc.blockStates[h] = blockStateNew
	}
	for h, pid := range params.pending {
		sc.blockStates[h] = blockStatePending
		sc.pendingBlocks[h] = pid
	}
	for h, tm := range params.pendingTime {
		sc.pendingTime[h] = tm
	}
	for h, pid := range params.received {
		sc.blockStates[h] = blockStateReceived
		sc.receivedBlocks[h] = pid
	}

	sc.peers = peers
	sc.peerTimeout = params.peerTimeout
	if params.syncTimeout == 0 {
		sc.syncTimeout = 10 * time.Second
	} else {
		sc.syncTimeout = params.syncTimeout
	}

	if params.targetPending == 0 {
		sc.targetPending = 10
	} else {
		sc.targetPending = params.targetPending
	}

	sc.minRecvRate = params.minRecvRate

	verifyScheduler(sc)

	return sc
}

func TestScInit(t *testing.T) {
	var (
		initHeight int64 = 5
		sc               = newScheduler(initHeight, time.Now())
	)
	assert.Equal(t, blockStateProcessed, sc.getStateAtHeight(initHeight))
	assert.Equal(t, blockStateUnknown, sc.getStateAtHeight(initHeight+1))
}

func TestScMaxHeights(t *testing.T) {

	tests := []struct {
		name    string
		sc      scheduler
		wantMax int64
	}{
		{
			name:    "no peers",
			sc:      scheduler{height: 11},
			wantMax: 10,
		},
		{
			name: "one ready peer",
			sc: scheduler{
				initHeight: 2,
				height:     3,
				peers:      map[p2p.ID]*scPeer{"P1": {height: 6, state: peerStateReady}},
			},
			wantMax: 6,
		},
		{
			name: "ready and removed peers",
			sc: scheduler{
				height: 1,
				peers: map[p2p.ID]*scPeer{
					"P1": {height: 4, state: peerStateReady},
					"P2": {height: 10, state: peerStateRemoved}},
			},
			wantMax: 4,
		},
		{
			name: "removed peers",
			sc: scheduler{
				height: 1,
				peers: map[p2p.ID]*scPeer{
					"P1": {height: 4, state: peerStateRemoved},
					"P2": {height: 10, state: peerStateRemoved}},
			},
			wantMax: 0,
		},
		{
			name: "new peers",
			sc: scheduler{
				height: 1,
				peers: map[p2p.ID]*scPeer{
					"P1": {height: -1, state: peerStateNew},
					"P2": {height: -1, state: peerStateNew}},
			},
			wantMax: 0,
		},
		{
			name: "mixed peers",
			sc: scheduler{
				height: 1,
				peers: map[p2p.ID]*scPeer{
					"P1": {height: -1, state: peerStateNew},
					"P2": {height: 10, state: peerStateReady},
					"P3": {height: 20, state: peerStateRemoved},
					"P4": {height: 22, state: peerStateReady},
				},
			},
			wantMax: 22,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			// maxHeight() should not mutate the scheduler
			wantSc := tt.sc

			resMax := tt.sc.maxHeight()
			assert.Equal(t, tt.wantMax, resMax)
			assert.Equal(t, wantSc, tt.sc)
		})
	}
}

func TestScAddPeer(t *testing.T) {

	type args struct {
		peerID p2p.ID
	}
	tests := []struct {
		name       string
		fields     scTestParams
		args       args
		wantFields scTestParams
		wantErr    bool
	}{
		{
			name:       "add first peer",
			fields:     scTestParams{},
			args:       args{peerID: "P1"},
			wantFields: scTestParams{peers: map[string]*scPeer{"P1": {height: -1, state: peerStateNew}}},
		},
		{
			name:   "add second peer",
			fields: scTestParams{peers: map[string]*scPeer{"P1": {height: -1, state: peerStateNew}}},
			args:   args{peerID: "P2"},
			wantFields: scTestParams{peers: map[string]*scPeer{
				"P1": {height: -1, state: peerStateNew},
				"P2": {height: -1, state: peerStateNew}}},
		},
		{
			name:       "attempt to add duplicate peer",
			fields:     scTestParams{peers: map[string]*scPeer{"P1": {height: -1}}},
			args:       args{peerID: "P1"},
			wantFields: scTestParams{peers: map[string]*scPeer{"P1": {height: -1}}},
			wantErr:    true,
		},
		{
			name: "attempt to add duplicate peer with existing peer in Ready state",
			fields: scTestParams{
				peers: map[string]*scPeer{"P1": {state: peerStateReady, height: 3}},
				allB:  []int64{1, 2, 3},
			},
			args:    args{peerID: "P1"},
			wantErr: true,
			wantFields: scTestParams{
				peers: map[string]*scPeer{"P1": {state: peerStateReady, height: 3}},
				allB:  []int64{1, 2, 3},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			sc := newTestScheduler(tt.fields)
			if err := sc.addPeer(tt.args.peerID); (err != nil) != tt.wantErr {
				t.Errorf("scAddPeer() wantErr %v, error = %v", tt.wantErr, err)
			}
			wantSc := newTestScheduler(tt.wantFields)
			assert.Equal(t, wantSc, sc, "wanted peers %v, got %v", wantSc.peers, sc.peers)
		})
	}
}

func TestScTouchPeer(t *testing.T) {
	now := time.Now()

	type args struct {
		peerID p2p.ID
		time   time.Time
	}

	tests := []struct {
		name       string
		fields     scTestParams
		args       args
		wantFields scTestParams
		wantErr    bool
	}{
		{
			name: "attempt to touch non existing peer",
			fields: scTestParams{
				peers: map[string]*scPeer{"P1": {state: peerStateReady, height: 5}},
				allB:  []int64{1, 2, 3, 4, 5},
			},
			args: args{peerID: "P2", time: now},
			wantFields: scTestParams{peers: map[string]*scPeer{"P1": {state: peerStateReady, height: 5}},
				allB: []int64{1, 2, 3, 4, 5},
			},
			wantErr: true,
		},
		{
			name:       "attempt to touch peer in state New",
			fields:     scTestParams{peers: map[string]*scPeer{"P1": {}}},
			args:       args{peerID: "P1", time: now},
			wantFields: scTestParams{peers: map[string]*scPeer{"P1": {}}},
			wantErr:    true,
		},
		{
			name:       "attempt to touch peer in state Removed",
			fields:     scTestParams{peers: map[string]*scPeer{"P1": {state: peerStateRemoved}, "P2": {state: peerStateReady}}},
			args:       args{peerID: "P1", time: now},
			wantFields: scTestParams{peers: map[string]*scPeer{"P1": {state: peerStateRemoved}, "P2": {state: peerStateReady}}},
			wantErr:    true,
		},
		{
			name:       "touch peer in state Ready",
			fields:     scTestParams{peers: map[string]*scPeer{"P1": {state: peerStateReady, lastTouched: now}}},
			args:       args{peerID: "P1", time: now.Add(3 * time.Second)},
			wantFields: scTestParams{peers: map[string]*scPeer{"P1": {state: peerStateReady, lastTouched: now.Add(3 * time.Second)}}},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			sc := newTestScheduler(tt.fields)
			if err := sc.touchPeer(tt.args.peerID, tt.args.time); (err != nil) != tt.wantErr {
				t.Errorf("touchPeer() wantErr %v, error = %v", tt.wantErr, err)
			}
			wantSc := newTestScheduler(tt.wantFields)
			assert.Equal(t, wantSc, sc, "wanted peers %v, got %v", wantSc.peers, sc.peers)
		})
	}
}

func TestScPrunablePeers(t *testing.T) {
	now := time.Now()

	type args struct {
		threshold time.Duration
		time      time.Time
		minSpeed  int64
	}

	tests := []struct {
		name       string
		fields     scTestParams
		args       args
		wantResult []p2p.ID
	}{
		{
			name:       "no peers",
			fields:     scTestParams{peers: map[string]*scPeer{}},
			args:       args{threshold: time.Second, time: now.Add(time.Second + time.Millisecond), minSpeed: 100},
			wantResult: []p2p.ID{},
		},
		{
			name: "mixed peers",
			fields: scTestParams{peers: map[string]*scPeer{
				// X - removed, active, fast
				"P1": {state: peerStateRemoved, lastTouched: now.Add(time.Second), lastRate: 101},
				// X - ready, active, fast
				"P2": {state: peerStateReady, lastTouched: now.Add(time.Second), lastRate: 101},
				// X - removed, active, equal
				"P3": {state: peerStateRemoved, lastTouched: now.Add(time.Second), lastRate: 100},
				// V - ready, inactive, equal
				"P4": {state: peerStateReady, lastTouched: now, lastRate: 100},
				// V - ready, inactive, slow
				"P5": {state: peerStateReady, lastTouched: now, lastRate: 99},
				//  V - ready, active, slow
				"P6": {state: peerStateReady, lastTouched: now.Add(time.Second), lastRate: 90},
			}},
			args:       args{threshold: time.Second, time: now.Add(time.Second + time.Millisecond), minSpeed: 100},
			wantResult: []p2p.ID{"P4", "P5", "P6"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			sc := newTestScheduler(tt.fields)
			// peersSlowerThan should not mutate the scheduler
			wantSc := sc
			res := sc.prunablePeers(tt.args.threshold, tt.args.minSpeed, tt.args.time)
			assert.Equal(t, tt.wantResult, res)
			assert.Equal(t, wantSc, sc)
		})
	}
}

func TestScRemovePeer(t *testing.T) {

	type args struct {
		peerID p2p.ID
	}
	tests := []struct {
		name       string
		fields     scTestParams
		args       args
		wantFields scTestParams
		wantErr    bool
	}{
		{
			name:       "remove non existing peer",
			fields:     scTestParams{peers: map[string]*scPeer{"P1": {height: -1}}},
			args:       args{peerID: "P2"},
			wantFields: scTestParams{peers: map[string]*scPeer{"P1": {height: -1}}},
			wantErr:    true,
		},
		{
			name:       "remove single New peer",
			fields:     scTestParams{peers: map[string]*scPeer{"P1": {height: -1}}},
			args:       args{peerID: "P1"},
			wantFields: scTestParams{peers: map[string]*scPeer{"P1": {height: -1, state: peerStateRemoved}}},
		},
		{
			name:       "remove one of two New peers",
			fields:     scTestParams{peers: map[string]*scPeer{"P1": {height: -1}, "P2": {height: -1}}},
			args:       args{peerID: "P1"},
			wantFields: scTestParams{peers: map[string]*scPeer{"P1": {height: -1, state: peerStateRemoved}, "P2": {height: -1}}},
		},
		{
			name: "remove one Ready peer, all peers removed",
			fields: scTestParams{
				peers: map[string]*scPeer{
					"P1": {height: 10, state: peerStateRemoved},
					"P2": {height: 5, state: peerStateReady}},
				allB: []int64{1, 2, 3, 4, 5},
			},
			args: args{peerID: "P2"},
			wantFields: scTestParams{peers: map[string]*scPeer{
				"P1": {height: 10, state: peerStateRemoved},
				"P2": {height: 5, state: peerStateRemoved}},
			},
		},
		{
			name: "attempt to remove already removed peer",
			fields: scTestParams{
				height: 8,
				peers: map[string]*scPeer{
					"P1": {height: 10, state: peerStateRemoved},
					"P2": {height: 11, state: peerStateReady}},
				allB: []int64{8, 9, 10, 11},
			},
			args: args{peerID: "P1"},
			wantFields: scTestParams{
				height: 8,
				peers: map[string]*scPeer{
					"P1": {height: 10, state: peerStateRemoved},
					"P2": {height: 11, state: peerStateReady}},
				allB: []int64{8, 9, 10, 11}},
			wantErr: true,
		},
		{
			name: "remove Ready peer with blocks requested",
			fields: scTestParams{
				peers:   map[string]*scPeer{"P1": {height: 3, state: peerStateReady}},
				allB:    []int64{1, 2, 3},
				pending: map[int64]p2p.ID{1: "P1"},
			},
			args: args{peerID: "P1"},
			wantFields: scTestParams{
				peers:   map[string]*scPeer{"P1": {height: 3, state: peerStateRemoved}},
				allB:    []int64{},
				pending: map[int64]p2p.ID{},
			},
		},
		{
			name: "remove Ready peer with blocks received",
			fields: scTestParams{
				peers:    map[string]*scPeer{"P1": {height: 3, state: peerStateReady}},
				allB:     []int64{1, 2, 3},
				received: map[int64]p2p.ID{1: "P1"},
			},
			args: args{peerID: "P1"},
			wantFields: scTestParams{
				peers:    map[string]*scPeer{"P1": {height: 3, state: peerStateRemoved}},
				allB:     []int64{},
				received: map[int64]p2p.ID{},
			},
		},
		{
			name: "remove Ready peer with blocks received and requested (not yet received)",
			fields: scTestParams{
				peers:    map[string]*scPeer{"P1": {height: 4, state: peerStateReady}},
				allB:     []int64{1, 2, 3, 4},
				pending:  map[int64]p2p.ID{1: "P1", 3: "P1"},
				received: map[int64]p2p.ID{2: "P1", 4: "P1"},
			},
			args: args{peerID: "P1"},
			wantFields: scTestParams{
				peers:    map[string]*scPeer{"P1": {height: 4, state: peerStateRemoved}},
				allB:     []int64{},
				pending:  map[int64]p2p.ID{},
				received: map[int64]p2p.ID{},
			},
		},
		{
			name: "remove Ready peer from multiple peers set, with blocks received and requested (not yet received)",
			fields: scTestParams{
				peers: map[string]*scPeer{
					"P1": {height: 6, state: peerStateReady},
					"P2": {height: 6, state: peerStateReady},
				},
				allB:     []int64{1, 2, 3, 4, 5, 6},
				pending:  map[int64]p2p.ID{1: "P1", 3: "P2", 6: "P1"},
				received: map[int64]p2p.ID{2: "P1", 4: "P2", 5: "P2"},
			},
			args: args{peerID: "P1"},
			wantFields: scTestParams{
				peers: map[string]*scPeer{
					"P1": {height: 6, state: peerStateRemoved},
					"P2": {height: 6, state: peerStateReady},
				},
				allB:     []int64{1, 2, 3, 4, 5, 6},
				pending:  map[int64]p2p.ID{3: "P2"},
				received: map[int64]p2p.ID{4: "P2", 5: "P2"},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			sc := newTestScheduler(tt.fields)
			if err := sc.removePeer(tt.args.peerID); (err != nil) != tt.wantErr {
				t.Errorf("removePeer() wantErr %v, error = %v", tt.wantErr, err)
			}
			wantSc := newTestScheduler(tt.wantFields)
			assert.Equal(t, wantSc, sc, "wanted peers %v, got %v", wantSc.peers, sc.peers)
		})
	}
}

func TestScSetPeerHeight(t *testing.T) {

	type args struct {
		peerID p2p.ID
		height int64
	}
	tests := []struct {
		name       string
		fields     scTestParams
		args       args
		wantFields scTestParams
		wantErr    bool
	}{
		{
			name: "change height of non existing peer",
			fields: scTestParams{
				peers: map[string]*scPeer{"P1": {height: 2, state: peerStateReady}},
				allB:  []int64{1, 2}},
			args: args{peerID: "P2", height: 4},
			wantFields: scTestParams{
				peers: map[string]*scPeer{"P1": {height: 2, state: peerStateReady}},
				allB:  []int64{1, 2}},
			wantErr: true,
		},
		{
			name: "increase height of removed peer",
			fields: scTestParams{
				peers: map[string]*scPeer{"P1": {height: 2, state: peerStateRemoved}}},
			args:       args{peerID: "P1", height: 4},
			wantFields: scTestParams{peers: map[string]*scPeer{"P1": {height: 2, state: peerStateRemoved}}},
			wantErr:    true,
		},
		{
			name: "decrease height of single peer",
			fields: scTestParams{
				peers: map[string]*scPeer{"P1": {height: 4, state: peerStateReady}},
				allB:  []int64{1, 2, 3, 4}},
			args: args{peerID: "P1", height: 2},
			wantFields: scTestParams{
				peers: map[string]*scPeer{"P1": {height: 4, state: peerStateRemoved}},
				allB:  []int64{}},
			wantErr: true,
		},
		{
			name: "increase height of single peer",
			fields: scTestParams{
				peers: map[string]*scPeer{"P1": {height: 2, state: peerStateReady}},
				allB:  []int64{1, 2}},
			args: args{peerID: "P1", height: 4},
			wantFields: scTestParams{
				peers: map[string]*scPeer{"P1": {height: 4, state: peerStateReady}},
				allB:  []int64{1, 2, 3, 4}},
		},
		{
			name: "noop height change of single peer",
			fields: scTestParams{
				peers: map[string]*scPeer{"P1": {height: 4, state: peerStateReady}},
				allB:  []int64{1, 2, 3, 4}},
			args: args{peerID: "P1", height: 4},
			wantFields: scTestParams{
				peers: map[string]*scPeer{"P1": {height: 4, state: peerStateReady}},
				allB:  []int64{1, 2, 3, 4}},
		},
		{
			name: "add peer with huge height 10**10 ",
			fields: scTestParams{
				peers:         map[string]*scPeer{"P2": {height: -1, state: peerStateNew}},
				targetPending: 4,
			},
			args: args{peerID: "P2", height: 10000000000},
			wantFields: scTestParams{
				targetPending: 4,
				peers:         map[string]*scPeer{"P2": {height: 10000000000, state: peerStateReady}},
				allB:          []int64{1, 2, 3, 4}},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			sc := newTestScheduler(tt.fields)
			if err := sc.setPeerHeight(tt.args.peerID, tt.args.height); (err != nil) != tt.wantErr {
				t.Errorf("setPeerHeight() wantErr %v, error = %v", tt.wantErr, err)
			}
			wantSc := newTestScheduler(tt.wantFields)
			assert.Equal(t, wantSc, sc, "wanted peers %v, got %v", wantSc.peers, sc.peers)
		})
	}
}

func TestScGetPeersAtHeight(t *testing.T) {

	type args struct {
		height int64
	}
	tests := []struct {
		name       string
		fields     scTestParams
		args       args
		wantResult []p2p.ID
	}{
		{
			name:       "no peers",
			fields:     scTestParams{peers: map[string]*scPeer{}},
			args:       args{height: 10},
			wantResult: []p2p.ID{},
		},
		{
			name:       "only new peers",
			fields:     scTestParams{peers: map[string]*scPeer{"P1": {height: -1, state: peerStateNew}}},
			args:       args{height: 10},
			wantResult: []p2p.ID{},
		},
		{
			name:       "only Removed peers",
			fields:     scTestParams{peers: map[string]*scPeer{"P1": {height: 4, state: peerStateRemoved}}},
			args:       args{height: 2},
			wantResult: []p2p.ID{},
		},
		{
			name: "one Ready shorter peer",
			fields: scTestParams{
				peers: map[string]*scPeer{"P1": {height: 4, state: peerStateReady}},
				allB:  []int64{1, 2, 3, 4},
			},
			args:       args{height: 5},
			wantResult: []p2p.ID{},
		},
		{
			name: "one Ready equal peer",
			fields: scTestParams{
				peers: map[string]*scPeer{"P1": {height: 4, state: peerStateReady}},
				allB:  []int64{1, 2, 3, 4},
			},
			args:       args{height: 4},
			wantResult: []p2p.ID{"P1"},
		},
		{
			name: "one Ready higher peer",
			fields: scTestParams{
				targetPending: 4,
				peers:         map[string]*scPeer{"P1": {height: 20, state: peerStateReady}},
				allB:          []int64{1, 2, 3, 4},
			},
			args:       args{height: 4},
			wantResult: []p2p.ID{"P1"},
		},
		{
			name: "multiple mixed peers",
			fields: scTestParams{
				height: 8,
				peers: map[string]*scPeer{
					"P1": {height: -1, state: peerStateNew},
					"P2": {height: 10, state: peerStateReady},
					"P3": {height: 5, state: peerStateReady},
					"P4": {height: 20, state: peerStateRemoved},
					"P5": {height: 11, state: peerStateReady}},
				allB: []int64{8, 9, 10, 11},
			},
			args:       args{height: 8},
			wantResult: []p2p.ID{"P2", "P5"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			sc := newTestScheduler(tt.fields)
			// getPeersAtHeight should not mutate the scheduler
			wantSc := sc
			res := sc.getPeersAtHeightOrAbove(tt.args.height)
			sort.Sort(PeerByID(res))
			assert.Equal(t, tt.wantResult, res)
			assert.Equal(t, wantSc, sc)
		})
	}
}

func TestScMarkPending(t *testing.T) {
	now := time.Now()

	type args struct {
		peerID p2p.ID
		height int64
		tm     time.Time
	}
	tests := []struct {
		name       string
		fields     scTestParams
		args       args
		wantFields scTestParams
		wantErr    bool
	}{
		{
			name: "attempt mark pending an unknown block",
			fields: scTestParams{
				peers: map[string]*scPeer{"P1": {height: 2, state: peerStateReady}},
				allB:  []int64{1, 2}},
			args: args{peerID: "P1", height: 3, tm: now},
			wantFields: scTestParams{
				peers: map[string]*scPeer{"P1": {height: 2, state: peerStateReady}},
				allB:  []int64{1, 2}},
			wantErr: true,
		},
		{
			name: "attempt mark pending from non existing peer",
			fields: scTestParams{
				peers: map[string]*scPeer{"P1": {height: 2, state: peerStateReady}},
				allB:  []int64{1, 2}},
			args: args{peerID: "P2", height: 1, tm: now},
			wantFields: scTestParams{
				peers: map[string]*scPeer{"P1": {height: 2, state: peerStateReady}},
				allB:  []int64{1, 2}},
			wantErr: true,
		},
		{
			name: "mark pending from Removed peer",
			fields: scTestParams{
				peers: map[string]*scPeer{"P1": {height: 2, state: peerStateRemoved}}},
			args: args{peerID: "P1", height: 1, tm: now},
			wantFields: scTestParams{
				peers: map[string]*scPeer{"P1": {height: 2, state: peerStateRemoved}}},
			wantErr: true,
		},
		{
			name: "mark pending from New peer",
			fields: scTestParams{
				peers: map[string]*scPeer{
					"P1": {height: 4, state: peerStateReady},
					"P2": {height: 4, state: peerStateNew},
				},
				allB: []int64{1, 2, 3, 4},
			},
			args: args{peerID: "P2", height: 2, tm: now},
			wantFields: scTestParams{
				peers: map[string]*scPeer{
					"P1": {height: 4, state: peerStateReady},
					"P2": {height: 4, state: peerStateNew},
				},
				allB: []int64{1, 2, 3, 4},
			},
			wantErr: true,
		},
		{
			name: "mark pending from short peer",
			fields: scTestParams{
				peers: map[string]*scPeer{
					"P1": {height: 4, state: peerStateReady},
					"P2": {height: 2, state: peerStateReady},
				},
				allB: []int64{1, 2, 3, 4},
			},
			args: args{peerID: "P2", height: 3, tm: now},
			wantFields: scTestParams{
				peers: map[string]*scPeer{
					"P1": {height: 4, state: peerStateReady},
					"P2": {height: 2, state: peerStateReady},
				},
				allB: []int64{1, 2, 3, 4},
			},
			wantErr: true,
		},
		{
			name: "mark pending all good",
			fields: scTestParams{
				peers:       map[string]*scPeer{"P1": {height: 2, state: peerStateReady}},
				allB:        []int64{1, 2},
				pending:     map[int64]p2p.ID{1: "P1"},
				pendingTime: map[int64]time.Time{1: now},
			},
			args: args{peerID: "P1", height: 2, tm: now.Add(time.Millisecond)},
			wantFields: scTestParams{
				peers:       map[string]*scPeer{"P1": {height: 2, state: peerStateReady}},
				allB:        []int64{1, 2},
				pending:     map[int64]p2p.ID{1: "P1", 2: "P1"},
				pendingTime: map[int64]time.Time{1: now, 2: now.Add(time.Millisecond)},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			sc := newTestScheduler(tt.fields)
			if err := sc.markPending(tt.args.peerID, tt.args.height, tt.args.tm); (err != nil) != tt.wantErr {
				t.Errorf("markPending() wantErr %v, error = %v", tt.wantErr, err)
			}
			wantSc := newTestScheduler(tt.wantFields)
			assert.Equal(t, wantSc, sc)
		})
	}
}

func TestScMarkReceived(t *testing.T) {
	now := time.Now()

	type args struct {
		peerID p2p.ID
		height int64
		size   int64
		tm     time.Time
	}
	tests := []struct {
		name       string
		fields     scTestParams
		args       args
		wantFields scTestParams
		wantErr    bool
	}{
		{
			name: "received from non existing peer",
			fields: scTestParams{
				peers: map[string]*scPeer{"P1": {height: 2, state: peerStateReady}},
				allB:  []int64{1, 2}},
			args: args{peerID: "P2", height: 1, size: 1000, tm: now},
			wantFields: scTestParams{
				peers: map[string]*scPeer{"P1": {height: 2, state: peerStateReady}},
				allB:  []int64{1, 2}},
			wantErr: true,
		},
		{
			name: "received from removed peer",
			fields: scTestParams{
				peers: map[string]*scPeer{"P1": {height: 2, state: peerStateRemoved}}},
			args: args{peerID: "P1", height: 1, size: 1000, tm: now},
			wantFields: scTestParams{
				peers: map[string]*scPeer{"P1": {height: 2, state: peerStateRemoved}}},
			wantErr: true,
		},
		{
			name: "received from unsolicited peer",
			fields: scTestParams{
				peers: map[string]*scPeer{
					"P1": {height: 4, state: peerStateReady},
					"P2": {height: 4, state: peerStateReady},
				},
				allB:    []int64{1, 2, 3, 4},
				pending: map[int64]p2p.ID{1: "P1", 2: "P2", 3: "P2", 4: "P1"},
			},
			args: args{peerID: "P1", height: 2, size: 1000, tm: now},
			wantFields: scTestParams{
				peers: map[string]*scPeer{
					"P1": {height: 4, state: peerStateReady},
					"P2": {height: 4, state: peerStateReady},
				},
				allB:    []int64{1, 2, 3, 4},
				pending: map[int64]p2p.ID{1: "P1", 2: "P2", 3: "P2", 4: "P1"},
			},
			wantErr: true,
		},
		{
			name: "received but blockRequest not sent",
			fields: scTestParams{
				peers:   map[string]*scPeer{"P1": {height: 4, state: peerStateReady}},
				allB:    []int64{1, 2, 3, 4},
				pending: map[int64]p2p.ID{},
			},
			args: args{peerID: "P1", height: 2, size: 1000, tm: now},
			wantFields: scTestParams{
				peers:   map[string]*scPeer{"P1": {height: 4, state: peerStateReady}},
				allB:    []int64{1, 2, 3, 4},
				pending: map[int64]p2p.ID{},
			},
			wantErr: true,
		},
		{
			name: "received with bad timestamp",
			fields: scTestParams{
				peers:       map[string]*scPeer{"P1": {height: 2, state: peerStateReady}},
				allB:        []int64{1, 2},
				pending:     map[int64]p2p.ID{1: "P1", 2: "P1"},
				pendingTime: map[int64]time.Time{1: now, 2: now.Add(time.Second)},
			},
			args: args{peerID: "P1", height: 2, size: 1000, tm: now},
			wantFields: scTestParams{
				peers:       map[string]*scPeer{"P1": {height: 2, state: peerStateReady}},
				allB:        []int64{1, 2},
				pending:     map[int64]p2p.ID{1: "P1", 2: "P1"},
				pendingTime: map[int64]time.Time{1: now, 2: now.Add(time.Second)},
			},
			wantErr: true,
		},
		{
			name: "received all good",
			fields: scTestParams{
				peers:       map[string]*scPeer{"P1": {height: 2, state: peerStateReady}},
				allB:        []int64{1, 2},
				pending:     map[int64]p2p.ID{1: "P1", 2: "P1"},
				pendingTime: map[int64]time.Time{1: now, 2: now},
			},
			args: args{peerID: "P1", height: 2, size: 1000, tm: now.Add(time.Millisecond)},
			wantFields: scTestParams{
				peers:       map[string]*scPeer{"P1": {height: 2, state: peerStateReady}},
				allB:        []int64{1, 2},
				pending:     map[int64]p2p.ID{1: "P1"},
				pendingTime: map[int64]time.Time{1: now},
				received:    map[int64]p2p.ID{2: "P1"},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			sc := newTestScheduler(tt.fields)
			if err := sc.markReceived(tt.args.peerID, tt.args.height, tt.args.size, now.Add(time.Second)); (err != nil) != tt.wantErr {
				t.Errorf("markReceived() wantErr %v, error = %v", tt.wantErr, err)
			}
			wantSc := newTestScheduler(tt.wantFields)
			assert.Equal(t, wantSc, sc)
		})
	}
}

func TestScMarkProcessed(t *testing.T) {
	now := time.Now()

	type args struct {
		height int64
	}
	tests := []struct {
		name       string
		fields     scTestParams
		args       args
		wantFields scTestParams
		wantErr    bool
	}{
		{
			name: "processed an unreceived block",
			fields: scTestParams{
				peers:       map[string]*scPeer{"P1": {height: 2, state: peerStateReady}},
				allB:        []int64{1, 2},
				pending:     map[int64]p2p.ID{2: "P1"},
				pendingTime: map[int64]time.Time{2: now},
				received:    map[int64]p2p.ID{1: "P1"}},
			args: args{height: 2},
			wantFields: scTestParams{
				peers:       map[string]*scPeer{"P1": {height: 2, state: peerStateReady}},
				allB:        []int64{1, 2},
				pending:     map[int64]p2p.ID{2: "P1"},
				pendingTime: map[int64]time.Time{2: now},
				received:    map[int64]p2p.ID{1: "P1"}},
			wantErr: true,
		},
		{
			name: "mark processed success",
			fields: scTestParams{
				height:      1,
				peers:       map[string]*scPeer{"P1": {height: 2, state: peerStateReady}},
				allB:        []int64{1, 2},
				pending:     map[int64]p2p.ID{2: "P1"},
				pendingTime: map[int64]time.Time{2: now},
				received:    map[int64]p2p.ID{1: "P1"}},
			args: args{height: 1},
			wantFields: scTestParams{
				height:      2,
				peers:       map[string]*scPeer{"P1": {height: 2, state: peerStateReady}},
				allB:        []int64{2},
				pending:     map[int64]p2p.ID{2: "P1"},
				pendingTime: map[int64]time.Time{2: now}},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			sc := newTestScheduler(tt.fields)
			oldBlockState := sc.getStateAtHeight(tt.args.height)
			if err := sc.markProcessed(tt.args.height); (err != nil) != tt.wantErr {
				t.Errorf("markProcessed() wantErr %v, error = %v", tt.wantErr, err)
			}
			if tt.wantErr {
				assert.Equal(t, oldBlockState, sc.getStateAtHeight(tt.args.height))
			} else {
				assert.Equal(t, blockStateProcessed, sc.getStateAtHeight(tt.args.height))
			}
			wantSc := newTestScheduler(tt.wantFields)
			checkSameScheduler(t, wantSc, sc)
		})
	}
}

func TestScAllBlocksProcessed(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name       string
		fields     scTestParams
		wantResult bool
	}{
		{
			name:       "no blocks, no peers",
			fields:     scTestParams{},
			wantResult: false,
		},
		{
			name: "only New blocks",
			fields: scTestParams{
				peers: map[string]*scPeer{"P1": {height: 4, state: peerStateReady}},
				allB:  []int64{1, 2, 3, 4},
			},
			wantResult: false,
		},
		{
			name: "only Pending blocks",
			fields: scTestParams{
				peers:       map[string]*scPeer{"P1": {height: 4, state: peerStateReady}},
				allB:        []int64{1, 2, 3, 4},
				pending:     map[int64]p2p.ID{1: "P1", 2: "P1", 3: "P1", 4: "P1"},
				pendingTime: map[int64]time.Time{1: now, 2: now, 3: now, 4: now},
			},
			wantResult: false,
		},
		{
			name: "only Received blocks",
			fields: scTestParams{
				peers:    map[string]*scPeer{"P1": {height: 4, state: peerStateReady}},
				allB:     []int64{1, 2, 3, 4},
				received: map[int64]p2p.ID{1: "P1", 2: "P1", 3: "P1", 4: "P1"},
			},
			wantResult: false,
		},
		{
			name: "only Processed blocks plus highest is received",
			fields: scTestParams{
				height: 4,
				peers: map[string]*scPeer{
					"P1": {height: 4, state: peerStateReady}},
				allB:     []int64{4},
				received: map[int64]p2p.ID{4: "P1"},
			},
			wantResult: true,
		},
		{
			name: "mixed block states",
			fields: scTestParams{
				peers:       map[string]*scPeer{"P1": {height: 4, state: peerStateReady}},
				allB:        []int64{1, 2, 3, 4},
				pending:     map[int64]p2p.ID{2: "P1", 4: "P1"},
				pendingTime: map[int64]time.Time{2: now, 4: now},
			},
			wantResult: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			sc := newTestScheduler(tt.fields)
			// allBlocksProcessed() should not mutate the scheduler
			wantSc := sc
			res := sc.allBlocksProcessed()
			assert.Equal(t, tt.wantResult, res)
			checkSameScheduler(t, wantSc, sc)
		})
	}
}

func TestScNextHeightToSchedule(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name       string
		fields     scTestParams
		wantHeight int64
	}{
		{
			name:       "no blocks",
			fields:     scTestParams{initHeight: 10, height: 11},
			wantHeight: -1,
		},
		{
			name: "only New blocks",
			fields: scTestParams{
				initHeight: 2,
				height:     3,
				peers:      map[string]*scPeer{"P1": {height: 6, state: peerStateReady}},
				allB:       []int64{3, 4, 5, 6},
			},
			wantHeight: 3,
		},
		{
			name: "only Pending blocks",
			fields: scTestParams{
				height:      1,
				peers:       map[string]*scPeer{"P1": {height: 4, state: peerStateReady}},
				allB:        []int64{1, 2, 3, 4},
				pending:     map[int64]p2p.ID{1: "P1", 2: "P1", 3: "P1", 4: "P1"},
				pendingTime: map[int64]time.Time{1: now, 2: now, 3: now, 4: now},
			},
			wantHeight: -1,
		},
		{
			name: "only Received blocks",
			fields: scTestParams{
				height:   1,
				peers:    map[string]*scPeer{"P1": {height: 4, state: peerStateReady}},
				allB:     []int64{1, 2, 3, 4},
				received: map[int64]p2p.ID{1: "P1", 2: "P1", 3: "P1", 4: "P1"},
			},
			wantHeight: -1,
		},
		{
			name: "only Processed blocks",
			fields: scTestParams{
				height: 1,
				peers:  map[string]*scPeer{"P1": {height: 4, state: peerStateReady}},
				allB:   []int64{1, 2, 3, 4},
			},
			wantHeight: 1,
		},
		{
			name: "mixed block states",
			fields: scTestParams{
				height:      1,
				peers:       map[string]*scPeer{"P1": {height: 4, state: peerStateReady}},
				allB:        []int64{1, 2, 3, 4},
				pending:     map[int64]p2p.ID{2: "P1"},
				pendingTime: map[int64]time.Time{2: now},
			},
			wantHeight: 1,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			sc := newTestScheduler(tt.fields)
			// nextHeightToSchedule() should not mutate the scheduler
			wantSc := sc

			resMin := sc.nextHeightToSchedule()
			assert.Equal(t, tt.wantHeight, resMin)
			checkSameScheduler(t, wantSc, sc)
		})
	}
}

func TestScSelectPeer(t *testing.T) {

	type args struct {
		height int64
	}
	tests := []struct {
		name       string
		fields     scTestParams
		args       args
		wantResult p2p.ID
		wantError  bool
	}{
		{
			name:       "no peers",
			fields:     scTestParams{peers: map[string]*scPeer{}},
			args:       args{height: 10},
			wantResult: "",
			wantError:  true,
		},
		{
			name:       "only new peers",
			fields:     scTestParams{peers: map[string]*scPeer{"P1": {height: -1, state: peerStateNew}}},
			args:       args{height: 10},
			wantResult: "",
			wantError:  true,
		},
		{
			name:       "only Removed peers",
			fields:     scTestParams{peers: map[string]*scPeer{"P1": {height: 4, state: peerStateRemoved}}},
			args:       args{height: 2},
			wantResult: "",
			wantError:  true,
		},
		{
			name: "one Ready shorter peer",
			fields: scTestParams{
				peers: map[string]*scPeer{"P1": {height: 4, state: peerStateReady}},
				allB:  []int64{1, 2, 3, 4},
			},
			args:       args{height: 5},
			wantResult: "",
			wantError:  true,
		},
		{
			name: "one Ready equal peer",
			fields: scTestParams{peers: map[string]*scPeer{"P1": {height: 4, state: peerStateReady}},
				allB: []int64{1, 2, 3, 4},
			},
			args:       args{height: 4},
			wantResult: "P1",
		},
		{
			name: "one Ready higher peer",
			fields: scTestParams{peers: map[string]*scPeer{"P1": {height: 6, state: peerStateReady}},
				allB: []int64{1, 2, 3, 4, 5, 6},
			},
			args:       args{height: 4},
			wantResult: "P1",
		},
		{
			name: "many Ready higher peers with different number of pending requests",
			fields: scTestParams{
				height: 4,
				peers: map[string]*scPeer{
					"P1": {height: 8, state: peerStateReady},
					"P2": {height: 9, state: peerStateReady}},
				allB: []int64{4, 5, 6, 7, 8, 9},
				pending: map[int64]p2p.ID{
					4: "P1", 6: "P1",
					5: "P2",
				},
			},
			args:       args{height: 4},
			wantResult: "P2",
		},
		{
			name: "many Ready higher peers with same number of pending requests",
			fields: scTestParams{
				peers: map[string]*scPeer{
					"P2": {height: 20, state: peerStateReady},
					"P1": {height: 15, state: peerStateReady},
					"P3": {height: 15, state: peerStateReady}},
				allB: []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
				pending: map[int64]p2p.ID{
					1: "P1", 2: "P1",
					3: "P3", 4: "P3",
					5: "P2", 6: "P2",
				},
			},
			args:       args{height: 7},
			wantResult: "P1",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			sc := newTestScheduler(tt.fields)
			// selectPeer should not mutate the scheduler
			wantSc := sc
			res, err := sc.selectPeer(tt.args.height)
			assert.Equal(t, tt.wantResult, res)
			assert.Equal(t, tt.wantError, err != nil)
			checkSameScheduler(t, wantSc, sc)
		})
	}
}

// makeScBlock makes an empty block.
func makeScBlock(height int64) *types.Block {
	return &types.Block{Header: types.Header{Height: height}}
}

// used in place of assert.Equal(t, want, actual) to avoid failures due to
// scheduler.lastAdvanced timestamp inequalities.
func checkSameScheduler(t *testing.T, want *scheduler, actual *scheduler) {
	assert.Equal(t, want.initHeight, actual.initHeight)
	assert.Equal(t, want.height, actual.height)
	assert.Equal(t, want.peers, actual.peers)
	assert.Equal(t, want.blockStates, actual.blockStates)
	assert.Equal(t, want.pendingBlocks, actual.pendingBlocks)
	assert.Equal(t, want.pendingTime, actual.pendingTime)
	assert.Equal(t, want.blockStates, actual.blockStates)
	assert.Equal(t, want.receivedBlocks, actual.receivedBlocks)
	assert.Equal(t, want.blockStates, actual.blockStates)
}

// checkScResults checks scheduler handler test results
func checkScResults(t *testing.T, wantErr bool, err error, wantEvent Event, event Event) {
	if (err != nil) != wantErr {
		t.Errorf("error = %v, wantErr %v", err, wantErr)
		return
	}
	switch wantEvent := wantEvent.(type) {
	case scPeerError:
		assert.Equal(t, wantEvent.peerID, event.(scPeerError).peerID)
		assert.Equal(t, wantEvent.reason != nil, event.(scPeerError).reason != nil)
	case scBlockReceived:
		assert.Equal(t, wantEvent.peerID, event.(scBlockReceived).peerID)
		assert.Equal(t, wantEvent.block, event.(scBlockReceived).block)
	case scSchedulerFail:
		assert.Equal(t, wantEvent.reason != nil, event.(scSchedulerFail).reason != nil)
	}
}

func TestScHandleBlockResponse(t *testing.T) {
	now := time.Now()
	block6FromP1 := bcBlockResponse{
		time:   now.Add(time.Millisecond),
		peerID: p2p.ID("P1"),
		size:   100,
		block:  makeScBlock(6),
	}

	type args struct {
		event bcBlockResponse
	}

	tests := []struct {
		name      string
		fields    scTestParams
		args      args
		wantEvent Event
		wantErr   bool
	}{
		{
			name:      "empty scheduler",
			fields:    scTestParams{},
			args:      args{event: block6FromP1},
			wantEvent: scPeerError{peerID: "P1", reason: fmt.Errorf("some error")},
		},
		{
			name:      "block from removed peer",
			fields:    scTestParams{peers: map[string]*scPeer{"P1": {height: 8, state: peerStateRemoved}}},
			args:      args{event: block6FromP1},
			wantEvent: scPeerError{peerID: "P1", reason: fmt.Errorf("some error")},
		},
		{
			name: "block we haven't asked for",
			fields: scTestParams{
				peers: map[string]*scPeer{"P1": {height: 8, state: peerStateReady}},
				allB:  []int64{1, 2, 3, 4, 5, 6, 7, 8}},
			args:      args{event: block6FromP1},
			wantEvent: scPeerError{peerID: "P1", reason: fmt.Errorf("some error")},
		},
		{
			name: "block from wrong peer",
			fields: scTestParams{
				peers:       map[string]*scPeer{"P2": {height: 8, state: peerStateReady}},
				allB:        []int64{1, 2, 3, 4, 5, 6, 7, 8},
				pending:     map[int64]p2p.ID{6: "P2"},
				pendingTime: map[int64]time.Time{6: now},
			},
			args:      args{event: block6FromP1},
			wantEvent: scPeerError{peerID: "P1", reason: fmt.Errorf("some error")},
		},
		{
			name: "block with bad timestamp",
			fields: scTestParams{
				peers:       map[string]*scPeer{"P1": {height: 8, state: peerStateReady}},
				allB:        []int64{1, 2, 3, 4, 5, 6, 7, 8},
				pending:     map[int64]p2p.ID{6: "P1"},
				pendingTime: map[int64]time.Time{6: now.Add(time.Second)},
			},
			args:      args{event: block6FromP1},
			wantEvent: scPeerError{peerID: "P1", reason: fmt.Errorf("some error")},
		},
		{
			name: "good block, accept",
			fields: scTestParams{
				peers:       map[string]*scPeer{"P1": {height: 8, state: peerStateReady}},
				allB:        []int64{1, 2, 3, 4, 5, 6, 7, 8},
				pending:     map[int64]p2p.ID{6: "P1"},
				pendingTime: map[int64]time.Time{6: now},
			},
			args:      args{event: block6FromP1},
			wantEvent: scBlockReceived{peerID: "P1", block: makeScBlock(6)},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			sc := newTestScheduler(tt.fields)
			event, err := sc.handleBlockResponse(tt.args.event)
			checkScResults(t, tt.wantErr, err, tt.wantEvent, event)
		})
	}
}

func TestScHandleNoBlockResponse(t *testing.T) {
	now := time.Now()
	noBlock6FromP1 := bcNoBlockResponse{
		time:   now.Add(time.Millisecond),
		peerID: p2p.ID("P1"),
		height: 6,
	}

	tests := []struct {
		name       string
		fields     scTestParams
		wantEvent  Event
		wantFields scTestParams
		wantErr    bool
	}{
		{
			name:       "empty scheduler",
			fields:     scTestParams{},
			wantEvent:  noOpEvent{},
			wantFields: scTestParams{},
		},
		{
			name:       "noBlock from removed peer",
			fields:     scTestParams{peers: map[string]*scPeer{"P1": {height: 8, state: peerStateRemoved}}},
			wantEvent:  noOpEvent{},
			wantFields: scTestParams{peers: map[string]*scPeer{"P1": {height: 8, state: peerStateRemoved}}},
		},
		{
			name: "for block we haven't asked for",
			fields: scTestParams{
				peers: map[string]*scPeer{"P1": {height: 8, state: peerStateReady}},
				allB:  []int64{1, 2, 3, 4, 5, 6, 7, 8}},
			wantEvent:  scPeerError{peerID: "P1", reason: fmt.Errorf("some error")},
			wantFields: scTestParams{peers: map[string]*scPeer{"P1": {height: 8, state: peerStateRemoved}}},
		},
		{
			name: "noBlock from peer we don't have",
			fields: scTestParams{
				peers:       map[string]*scPeer{"P2": {height: 8, state: peerStateReady}},
				allB:        []int64{1, 2, 3, 4, 5, 6, 7, 8},
				pending:     map[int64]p2p.ID{6: "P2"},
				pendingTime: map[int64]time.Time{6: now},
			},
			wantEvent: noOpEvent{},
			wantFields: scTestParams{
				peers:       map[string]*scPeer{"P2": {height: 8, state: peerStateReady}},
				allB:        []int64{1, 2, 3, 4, 5, 6, 7, 8},
				pending:     map[int64]p2p.ID{6: "P2"},
				pendingTime: map[int64]time.Time{6: now},
			},
		},
		{
			name: "noBlock from existing peer",
			fields: scTestParams{
				peers:       map[string]*scPeer{"P1": {height: 8, state: peerStateReady}},
				allB:        []int64{1, 2, 3, 4, 5, 6, 7, 8},
				pending:     map[int64]p2p.ID{6: "P1"},
				pendingTime: map[int64]time.Time{6: now},
			},
			wantEvent:  scPeerError{peerID: "P1", reason: fmt.Errorf("some error")},
			wantFields: scTestParams{peers: map[string]*scPeer{"P1": {height: 8, state: peerStateRemoved}}},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			sc := newTestScheduler(tt.fields)
			event, err := sc.handleNoBlockResponse(noBlock6FromP1)
			checkScResults(t, tt.wantErr, err, tt.wantEvent, event)
			wantSc := newTestScheduler(tt.wantFields)
			assert.Equal(t, wantSc, sc)
		})
	}
}

func TestScHandleBlockProcessed(t *testing.T) {
	now := time.Now()
	processed6FromP1 := pcBlockProcessed{
		peerID: p2p.ID("P1"),
		height: 6,
	}

	type args struct {
		event pcBlockProcessed
	}

	tests := []struct {
		name      string
		fields    scTestParams
		args      args
		wantEvent Event
		wantErr   bool
	}{
		{
			name:      "empty scheduler",
			fields:    scTestParams{height: 6},
			args:      args{event: processed6FromP1},
			wantEvent: scSchedulerFail{reason: fmt.Errorf("some error")},
		},
		{
			name: "processed block we don't have",
			fields: scTestParams{
				initHeight:  5,
				height:      6,
				peers:       map[string]*scPeer{"P1": {height: 8, state: peerStateReady}},
				allB:        []int64{6, 7, 8},
				pending:     map[int64]p2p.ID{6: "P1"},
				pendingTime: map[int64]time.Time{6: now},
			},
			args:      args{event: processed6FromP1},
			wantEvent: scSchedulerFail{reason: fmt.Errorf("some error")},
		},
		{
			name: "processed block ok, we processed all blocks",
			fields: scTestParams{
				initHeight: 5,
				height:     6,
				peers:      map[string]*scPeer{"P1": {height: 7, state: peerStateReady}},
				allB:       []int64{6, 7},
				received:   map[int64]p2p.ID{6: "P1", 7: "P1"},
			},
			args:      args{event: processed6FromP1},
			wantEvent: scFinishedEv{},
		},
		{
			name: "processed block ok, we still have blocks to process",
			fields: scTestParams{
				initHeight: 5,
				height:     6,
				peers:      map[string]*scPeer{"P1": {height: 8, state: peerStateReady}},
				allB:       []int64{6, 7, 8},
				pending:    map[int64]p2p.ID{7: "P1", 8: "P1"},
				received:   map[int64]p2p.ID{6: "P1"},
			},
			args:      args{event: processed6FromP1},
			wantEvent: noOpEvent{},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			sc := newTestScheduler(tt.fields)
			event, err := sc.handleBlockProcessed(tt.args.event)
			checkScResults(t, tt.wantErr, err, tt.wantEvent, event)
		})
	}
}

func TestScHandleBlockVerificationFailure(t *testing.T) {
	now := time.Now()

	type args struct {
		event pcBlockVerificationFailure
	}

	tests := []struct {
		name      string
		fields    scTestParams
		args      args
		wantEvent Event
		wantErr   bool
	}{
		{
			name:      "empty scheduler",
			fields:    scTestParams{},
			args:      args{event: pcBlockVerificationFailure{height: 10, firstPeerID: "P1", secondPeerID: "P1"}},
			wantEvent: noOpEvent{},
		},
		{
			name: "failed block we don't have, single peer is still removed",
			fields: scTestParams{
				initHeight:  5,
				height:      6,
				peers:       map[string]*scPeer{"P1": {height: 8, state: peerStateReady}},
				allB:        []int64{6, 7, 8},
				pending:     map[int64]p2p.ID{6: "P1"},
				pendingTime: map[int64]time.Time{6: now},
			},
			args:      args{event: pcBlockVerificationFailure{height: 10, firstPeerID: "P1", secondPeerID: "P1"}},
			wantEvent: scFinishedEv{},
		},
		{
			name: "failed block we don't have, one of two peers are removed",
			fields: scTestParams{
				initHeight:  5,
				peers:       map[string]*scPeer{"P1": {height: 8, state: peerStateReady}, "P2": {height: 8, state: peerStateReady}},
				allB:        []int64{6, 7, 8},
				pending:     map[int64]p2p.ID{6: "P1"},
				pendingTime: map[int64]time.Time{6: now},
			},
			args:      args{event: pcBlockVerificationFailure{height: 10, firstPeerID: "P1", secondPeerID: "P1"}},
			wantEvent: noOpEvent{},
		},
		{
			name: "failed block, all blocks are processed after removal",
			fields: scTestParams{
				initHeight: 5,
				height:     6,
				peers:      map[string]*scPeer{"P1": {height: 7, state: peerStateReady}},
				allB:       []int64{6, 7},
				received:   map[int64]p2p.ID{6: "P1", 7: "P1"},
			},
			args:      args{event: pcBlockVerificationFailure{height: 7, firstPeerID: "P1", secondPeerID: "P1"}},
			wantEvent: scFinishedEv{},
		},
		{
			name: "failed block, we still have blocks to process",
			fields: scTestParams{
				initHeight: 4,
				peers:      map[string]*scPeer{"P1": {height: 8, state: peerStateReady}, "P2": {height: 8, state: peerStateReady}},
				allB:       []int64{5, 6, 7, 8},
				pending:    map[int64]p2p.ID{7: "P1", 8: "P1"},
				received:   map[int64]p2p.ID{5: "P1", 6: "P1"},
			},
			args:      args{event: pcBlockVerificationFailure{height: 5, firstPeerID: "P1", secondPeerID: "P1"}},
			wantEvent: noOpEvent{},
		},
		{
			name: "failed block, H+1 and H+2 delivered by different peers, we still have blocks to process",
			fields: scTestParams{
				initHeight: 4,
				peers: map[string]*scPeer{
					"P1": {height: 8, state: peerStateReady},
					"P2": {height: 8, state: peerStateReady},
					"P3": {height: 8, state: peerStateReady},
				},
				allB:     []int64{5, 6, 7, 8},
				pending:  map[int64]p2p.ID{7: "P1", 8: "P1"},
				received: map[int64]p2p.ID{5: "P1", 6: "P1"},
			},
			args:      args{event: pcBlockVerificationFailure{height: 5, firstPeerID: "P1", secondPeerID: "P2"}},
			wantEvent: noOpEvent{},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			sc := newTestScheduler(tt.fields)
			event, err := sc.handleBlockProcessError(tt.args.event)
			checkScResults(t, tt.wantErr, err, tt.wantEvent, event)
		})
	}
}

func TestScHandleAddNewPeer(t *testing.T) {
	addP1 := bcAddNewPeer{
		peerID: p2p.ID("P1"),
	}
	type args struct {
		event bcAddNewPeer
	}

	tests := []struct {
		name      string
		fields    scTestParams
		args      args
		wantEvent Event
		wantErr   bool
	}{
		{
			name:      "add P1 to empty scheduler",
			fields:    scTestParams{},
			args:      args{event: addP1},
			wantEvent: noOpEvent{},
		},
		{
			name: "add duplicate peer",
			fields: scTestParams{
				height: 6,
				peers:  map[string]*scPeer{"P1": {height: 8, state: peerStateReady}},
				allB:   []int64{6, 7, 8},
			},
			args:      args{event: addP1},
			wantEvent: scSchedulerFail{reason: fmt.Errorf("some error")},
		},
		{
			name: "add P1 to non empty scheduler",
			fields: scTestParams{
				height: 6,
				peers:  map[string]*scPeer{"P2": {height: 8, state: peerStateReady}},
				allB:   []int64{6, 7, 8},
			},
			args:      args{event: addP1},
			wantEvent: noOpEvent{},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			sc := newTestScheduler(tt.fields)
			event, err := sc.handleAddNewPeer(tt.args.event)
			checkScResults(t, tt.wantErr, err, tt.wantEvent, event)
		})
	}
}

func TestScHandleTryPrunePeer(t *testing.T) {
	now := time.Now()

	pruneEv := rTryPrunePeer{
		time: now.Add(time.Second + time.Millisecond),
	}
	type args struct {
		event rTryPrunePeer
	}

	tests := []struct {
		name      string
		fields    scTestParams
		args      args
		wantEvent Event
		wantErr   bool
	}{
		{
			name:      "no peers",
			fields:    scTestParams{},
			args:      args{event: pruneEv},
			wantEvent: noOpEvent{},
		},
		{
			name: "no prunable peers",
			fields: scTestParams{
				minRecvRate: 100,
				peers: map[string]*scPeer{
					// X - removed, active, fast
					"P1": {state: peerStateRemoved, lastTouched: now.Add(time.Second), lastRate: 101},
					// X - ready, active, fast
					"P2": {state: peerStateReady, lastTouched: now.Add(time.Second), lastRate: 101},
					// X - removed, active, equal
					"P3": {state: peerStateRemoved, lastTouched: now.Add(time.Second), lastRate: 100}},
				peerTimeout: time.Second,
			},
			args:      args{event: pruneEv},
			wantEvent: noOpEvent{},
		},
		{
			name: "mixed peers",
			fields: scTestParams{
				minRecvRate: 100,
				peers: map[string]*scPeer{
					// X - removed, active, fast
					"P1": {state: peerStateRemoved, lastTouched: now.Add(time.Second), lastRate: 101, height: 5},
					// X - ready, active, fast
					"P2": {state: peerStateReady, lastTouched: now.Add(time.Second), lastRate: 101, height: 5},
					// X - removed, active, equal
					"P3": {state: peerStateRemoved, lastTouched: now.Add(time.Second), lastRate: 100, height: 5},
					// V - ready, inactive, equal
					"P4": {state: peerStateReady, lastTouched: now, lastRate: 100, height: 7},
					// V - ready, inactive, slow
					"P5": {state: peerStateReady, lastTouched: now, lastRate: 99, height: 7},
					//  V - ready, active, slow
					"P6": {state: peerStateReady, lastTouched: now.Add(time.Second), lastRate: 90, height: 7},
				},
				allB:        []int64{1, 2, 3, 4, 5, 6, 7},
				peerTimeout: time.Second},
			args:      args{event: pruneEv},
			wantEvent: scPeersPruned{peers: []p2p.ID{"P4", "P5", "P6"}},
		},
		{
			name: "mixed peers, finish after pruning",
			fields: scTestParams{
				minRecvRate: 100,
				height:      6,
				peers: map[string]*scPeer{
					// X - removed, active, fast
					"P1": {state: peerStateRemoved, lastTouched: now.Add(time.Second), lastRate: 101, height: 5},
					// X - ready, active, fast
					"P2": {state: peerStateReady, lastTouched: now.Add(time.Second), lastRate: 101, height: 5},
					// X - removed, active, equal
					"P3": {state: peerStateRemoved, lastTouched: now.Add(time.Second), lastRate: 100, height: 5},
					// V - ready, inactive, equal
					"P4": {state: peerStateReady, lastTouched: now, lastRate: 100, height: 7},
					// V - ready, inactive, slow
					"P5": {state: peerStateReady, lastTouched: now, lastRate: 99, height: 7},
					//  V - ready, active, slow
					"P6": {state: peerStateReady, lastTouched: now.Add(time.Second), lastRate: 90, height: 7},
				},
				allB:        []int64{6, 7},
				peerTimeout: time.Second},
			args:      args{event: pruneEv},
			wantEvent: scFinishedEv{},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			sc := newTestScheduler(tt.fields)
			event, err := sc.handleTryPrunePeer(tt.args.event)
			checkScResults(t, tt.wantErr, err, tt.wantEvent, event)
		})
	}
}

func TestScHandleTrySchedule(t *testing.T) {
	now := time.Now()
	tryEv := rTrySchedule{
		time: now.Add(time.Second + time.Millisecond),
	}

	type args struct {
		event rTrySchedule
	}
	tests := []struct {
		name      string
		fields    scTestParams
		args      args
		wantEvent Event
		wantErr   bool
	}{
		{
			name:      "no peers",
			fields:    scTestParams{startTime: now, peers: map[string]*scPeer{}},
			args:      args{event: tryEv},
			wantEvent: noOpEvent{},
		},
		{
			name:      "only new peers",
			fields:    scTestParams{startTime: now, peers: map[string]*scPeer{"P1": {height: -1, state: peerStateNew}}},
			args:      args{event: tryEv},
			wantEvent: noOpEvent{},
		},
		{
			name:      "only Removed peers",
			fields:    scTestParams{startTime: now, peers: map[string]*scPeer{"P1": {height: 4, state: peerStateRemoved}}},
			args:      args{event: tryEv},
			wantEvent: noOpEvent{},
		},
		{
			name: "one Ready shorter peer",
			fields: scTestParams{
				startTime: now,
				height:    6,
				peers:     map[string]*scPeer{"P1": {height: 4, state: peerStateReady}}},
			args:      args{event: tryEv},
			wantEvent: noOpEvent{},
		},
		{
			name: "one Ready equal peer",
			fields: scTestParams{
				startTime: now,
				peers:     map[string]*scPeer{"P1": {height: 4, state: peerStateReady}},
				allB:      []int64{1, 2, 3, 4}},
			args:      args{event: tryEv},
			wantEvent: scBlockRequest{peerID: "P1", height: 1},
		},
		{
			name: "many Ready higher peers with different number of pending requests",
			fields: scTestParams{
				startTime: now,
				peers: map[string]*scPeer{
					"P1": {height: 4, state: peerStateReady},
					"P2": {height: 5, state: peerStateReady}},
				allB: []int64{1, 2, 3, 4, 5},
				pending: map[int64]p2p.ID{
					1: "P1", 2: "P1",
					3: "P2",
				},
			},
			args:      args{event: tryEv},
			wantEvent: scBlockRequest{peerID: "P2", height: 4},
		},

		{
			name: "many Ready higher peers with same number of pending requests",
			fields: scTestParams{
				startTime: now,
				peers: map[string]*scPeer{
					"P2": {height: 8, state: peerStateReady},
					"P1": {height: 8, state: peerStateReady},
					"P3": {height: 8, state: peerStateReady}},
				allB: []int64{1, 2, 3, 4, 5, 6, 7, 8},
				pending: map[int64]p2p.ID{
					1: "P1", 2: "P1",
					3: "P3", 4: "P3",
					5: "P2", 6: "P2",
				},
			},
			args:      args{event: tryEv},
			wantEvent: scBlockRequest{peerID: "P1", height: 7},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			sc := newTestScheduler(tt.fields)
			event, err := sc.handleTrySchedule(tt.args.event)
			checkScResults(t, tt.wantErr, err, tt.wantEvent, event)
		})
	}
}

func TestScHandleStatusResponse(t *testing.T) {
	now := time.Now()
	statusRespP1Ev := bcStatusResponse{
		time:   now.Add(time.Second + time.Millisecond),
		peerID: "P1",
		height: 6,
	}

	type args struct {
		event bcStatusResponse
	}
	tests := []struct {
		name      string
		fields    scTestParams
		args      args
		wantEvent Event
		wantErr   bool
	}{
		{
			name: "change height of non existing peer",
			fields: scTestParams{
				peers: map[string]*scPeer{"P2": {height: 2, state: peerStateReady}},
				allB:  []int64{1, 2},
			},
			args:      args{event: statusRespP1Ev},
			wantEvent: scPeerError{peerID: "P1", reason: fmt.Errorf("some error")},
		},

		{
			name:      "increase height of removed peer",
			fields:    scTestParams{peers: map[string]*scPeer{"P1": {height: 2, state: peerStateRemoved}}},
			args:      args{event: statusRespP1Ev},
			wantEvent: scPeerError{peerID: "P1", reason: fmt.Errorf("some error")},
		},

		{
			name: "decrease height of single peer",
			fields: scTestParams{
				height: 5,
				peers:  map[string]*scPeer{"P1": {height: 10, state: peerStateReady}},
				allB:   []int64{5, 6, 7, 8, 9, 10},
			},
			args:      args{event: statusRespP1Ev},
			wantEvent: scPeerError{peerID: "P1", reason: fmt.Errorf("some error")},
		},

		{
			name: "increase height of single peer",
			fields: scTestParams{
				peers: map[string]*scPeer{"P1": {height: 2, state: peerStateReady}},
				allB:  []int64{1, 2}},
			args:      args{event: statusRespP1Ev},
			wantEvent: noOpEvent{},
		},
		{
			name: "noop height change of single peer",
			fields: scTestParams{
				peers: map[string]*scPeer{"P1": {height: 6, state: peerStateReady}},
				allB:  []int64{1, 2, 3, 4, 5, 6}},
			args:      args{event: statusRespP1Ev},
			wantEvent: noOpEvent{},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			sc := newTestScheduler(tt.fields)
			event, err := sc.handleStatusResponse(tt.args.event)
			checkScResults(t, tt.wantErr, err, tt.wantEvent, event)
		})
	}
}

func TestScHandle(t *testing.T) {
	now := time.Now()

	type unknownEv struct {
		priorityNormal
	}

	t0 := time.Now()
	tick := make([]time.Time, 100)
	for i := range tick {
		tick[i] = t0.Add(time.Duration(i) * time.Millisecond)
	}

	type args struct {
		event Event
	}
	type scStep struct {
		currentSc *scTestParams
		args      args
		wantEvent Event
		wantErr   bool
		wantSc    *scTestParams
	}
	tests := []struct {
		name  string
		steps []scStep
	}{
		{
			name: "unknown event",
			steps: []scStep{
				{ // add P1
					currentSc: &scTestParams{},
					args:      args{event: unknownEv{}},
					wantEvent: scSchedulerFail{reason: fmt.Errorf("some error")},
					wantSc:    &scTestParams{},
				},
			},
		},
		{
			name: "single peer, sync 3 blocks",
			steps: []scStep{
				{ // add P1
					currentSc: &scTestParams{startTime: now, peers: map[string]*scPeer{}, height: 1},
					args:      args{event: bcAddNewPeer{peerID: "P1"}},
					wantEvent: noOpEvent{},
					wantSc:    &scTestParams{startTime: now, peers: map[string]*scPeer{"P1": {height: -1, state: peerStateNew}}, height: 1},
				},
				{ // set height of P1
					args:      args{event: bcStatusResponse{peerID: "P1", time: tick[0], height: 3}},
					wantEvent: noOpEvent{},
					wantSc: &scTestParams{
						startTime: now,
						peers:     map[string]*scPeer{"P1": {height: 3, state: peerStateReady}},
						allB:      []int64{1, 2, 3},
						height:    1,
					},
				},
				{ // schedule block 1
					args:      args{event: rTrySchedule{time: tick[1]}},
					wantEvent: scBlockRequest{peerID: "P1", height: 1},
					wantSc: &scTestParams{
						startTime:   now,
						peers:       map[string]*scPeer{"P1": {height: 3, state: peerStateReady}},
						allB:        []int64{1, 2, 3},
						pending:     map[int64]p2p.ID{1: "P1"},
						pendingTime: map[int64]time.Time{1: tick[1]},
						height:      1,
					},
				},
				{ // schedule block 2
					args:      args{event: rTrySchedule{time: tick[2]}},
					wantEvent: scBlockRequest{peerID: "P1", height: 2},
					wantSc: &scTestParams{
						startTime:   now,
						peers:       map[string]*scPeer{"P1": {height: 3, state: peerStateReady}},
						allB:        []int64{1, 2, 3},
						pending:     map[int64]p2p.ID{1: "P1", 2: "P1"},
						pendingTime: map[int64]time.Time{1: tick[1], 2: tick[2]},
						height:      1,
					},
				},
				{ // schedule block 3
					args:      args{event: rTrySchedule{time: tick[3]}},
					wantEvent: scBlockRequest{peerID: "P1", height: 3},
					wantSc: &scTestParams{
						startTime:   now,
						peers:       map[string]*scPeer{"P1": {height: 3, state: peerStateReady}},
						allB:        []int64{1, 2, 3},
						pending:     map[int64]p2p.ID{1: "P1", 2: "P1", 3: "P1"},
						pendingTime: map[int64]time.Time{1: tick[1], 2: tick[2], 3: tick[3]},
						height:      1,
					},
				},
				{ // block response 1
					args:      args{event: bcBlockResponse{peerID: "P1", time: tick[4], size: 100, block: makeScBlock(1)}},
					wantEvent: scBlockReceived{peerID: "P1", block: makeScBlock(1)},
					wantSc: &scTestParams{
						startTime:   now,
						peers:       map[string]*scPeer{"P1": {height: 3, state: peerStateReady, lastTouched: tick[4]}},
						allB:        []int64{1, 2, 3},
						pending:     map[int64]p2p.ID{2: "P1", 3: "P1"},
						pendingTime: map[int64]time.Time{2: tick[2], 3: tick[3]},
						received:    map[int64]p2p.ID{1: "P1"},
						height:      1,
					},
				},
				{ // block response 2
					args:      args{event: bcBlockResponse{peerID: "P1", time: tick[5], size: 100, block: makeScBlock(2)}},
					wantEvent: scBlockReceived{peerID: "P1", block: makeScBlock(2)},
					wantSc: &scTestParams{
						startTime:   now,
						peers:       map[string]*scPeer{"P1": {height: 3, state: peerStateReady, lastTouched: tick[5]}},
						allB:        []int64{1, 2, 3},
						pending:     map[int64]p2p.ID{3: "P1"},
						pendingTime: map[int64]time.Time{3: tick[3]},
						received:    map[int64]p2p.ID{1: "P1", 2: "P1"},
						height:      1,
					},
				},
				{ // block response 3
					args:      args{event: bcBlockResponse{peerID: "P1", time: tick[6], size: 100, block: makeScBlock(3)}},
					wantEvent: scBlockReceived{peerID: "P1", block: makeScBlock(3)},
					wantSc: &scTestParams{
						startTime: now,
						peers:     map[string]*scPeer{"P1": {height: 3, state: peerStateReady, lastTouched: tick[6]}},
						allB:      []int64{1, 2, 3},
						received:  map[int64]p2p.ID{1: "P1", 2: "P1", 3: "P1"},
						height:    1,
					},
				},
				{ // processed block 1
					args:      args{event: pcBlockProcessed{peerID: p2p.ID("P1"), height: 1}},
					wantEvent: noOpEvent{},
					wantSc: &scTestParams{
						startTime: now,
						peers:     map[string]*scPeer{"P1": {height: 3, state: peerStateReady, lastTouched: tick[6]}},
						allB:      []int64{2, 3},
						received:  map[int64]p2p.ID{2: "P1", 3: "P1"},
						height:    2,
					},
				},
				{ // processed block 2
					args:      args{event: pcBlockProcessed{peerID: p2p.ID("P1"), height: 2}},
					wantEvent: scFinishedEv{},
					wantSc: &scTestParams{
						startTime: now,
						peers:     map[string]*scPeer{"P1": {height: 3, state: peerStateReady, lastTouched: tick[6]}},
						allB:      []int64{3},
						received:  map[int64]p2p.ID{3: "P1"},
						height:    3,
					},
				},
			},
		},
		{
			name: "block verification failure",
			steps: []scStep{
				{ // failure processing block 1
					currentSc: &scTestParams{
						startTime: now,
						peers: map[string]*scPeer{
							"P1": {height: 4, state: peerStateReady, lastTouched: tick[6]},
							"P2": {height: 3, state: peerStateReady, lastTouched: tick[6]}},
						allB:     []int64{1, 2, 3, 4},
						received: map[int64]p2p.ID{1: "P1", 2: "P1", 3: "P1"},
						height:   1,
					},
					args:      args{event: pcBlockVerificationFailure{height: 1, firstPeerID: "P1", secondPeerID: "P1"}},
					wantEvent: noOpEvent{},
					wantSc: &scTestParams{
						startTime: now,
						peers: map[string]*scPeer{
							"P1": {height: 4, state: peerStateRemoved, lastTouched: tick[6]},
							"P2": {height: 3, state: peerStateReady, lastTouched: tick[6]}},
						allB:     []int64{1, 2, 3},
						received: map[int64]p2p.ID{},
						height:   1,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			var sc *scheduler
			for i, step := range tt.steps {
				// First step must always initialise the currentState as state.
				if step.currentSc != nil {
					sc = newTestScheduler(*step.currentSc)
				}
				if sc == nil {
					panic("Bad (initial?) step")
				}

				nextEvent, err := sc.handle(step.args.event)
				wantSc := newTestScheduler(*step.wantSc)

				t.Logf("step %d(%v): %s", i, step.args.event, sc)
				checkSameScheduler(t, wantSc, sc)

				checkScResults(t, step.wantErr, err, step.wantEvent, nextEvent)

				// Next step may use the wantedState as their currentState.
				sc = newTestScheduler(*step.wantSc)
			}
		})
	}
}
