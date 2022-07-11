package evidence_test

import (
	"context"
	"encoding/hex"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/internal/eventbus"
	"github.com/tendermint/tendermint/internal/evidence"
	"github.com/tendermint/tendermint/internal/evidence/mocks"
	"github.com/tendermint/tendermint/internal/p2p"
	"github.com/tendermint/tendermint/internal/p2p/p2ptest"
	sm "github.com/tendermint/tendermint/internal/state"
	"github.com/tendermint/tendermint/libs/log"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

var (
	numEvidence = 10

	rng = rand.New(rand.NewSource(time.Now().UnixNano()))
)

type reactorTestSuite struct {
	network          *p2ptest.Network
	logger           log.Logger
	reactors         map[types.NodeID]*evidence.Reactor
	pools            map[types.NodeID]*evidence.Pool
	evidenceChannels map[types.NodeID]p2p.Channel
	peerUpdates      map[types.NodeID]*p2p.PeerUpdates
	peerChans        map[types.NodeID]chan p2p.PeerUpdate
	nodes            []*p2ptest.Node
	numStateStores   int
}

func setup(ctx context.Context, t *testing.T, stateStores []sm.Store) *reactorTestSuite {
	t.Helper()

	pID := make([]byte, 16)
	_, err := rng.Read(pID)
	require.NoError(t, err)

	numStateStores := len(stateStores)
	rts := &reactorTestSuite{
		numStateStores: numStateStores,
		logger:         log.NewNopLogger().With("testCase", t.Name()),
		network:        p2ptest.MakeNetwork(ctx, t, p2ptest.NetworkOptions{NumNodes: numStateStores}),
		reactors:       make(map[types.NodeID]*evidence.Reactor, numStateStores),
		pools:          make(map[types.NodeID]*evidence.Pool, numStateStores),
		peerUpdates:    make(map[types.NodeID]*p2p.PeerUpdates, numStateStores),
		peerChans:      make(map[types.NodeID]chan p2p.PeerUpdate, numStateStores),
	}

	chDesc := &p2p.ChannelDescriptor{ID: evidence.EvidenceChannel, MessageType: new(tmproto.Evidence)}
	rts.evidenceChannels = rts.network.MakeChannelsNoCleanup(ctx, t, chDesc)
	require.Len(t, rts.network.RandomNode().PeerManager.Peers(), 0)

	idx := 0
	evidenceTime := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)

	for nodeID := range rts.network.Nodes {
		logger := rts.logger.With("validator", idx)
		evidenceDB := dbm.NewMemDB()
		blockStore := &mocks.BlockStore{}
		state, _ := stateStores[idx].Load()
		blockStore.On("LoadBlockMeta", mock.AnythingOfType("int64")).Return(func(h int64) *types.BlockMeta {
			if h <= state.LastBlockHeight {
				return &types.BlockMeta{Header: types.Header{Time: evidenceTime}}
			}
			return nil
		})
		eventBus := eventbus.NewDefault(logger)
		err = eventBus.Start(ctx)
		require.NoError(t, err)

		rts.pools[nodeID] = evidence.NewPool(logger, evidenceDB, stateStores[idx], blockStore, evidence.NopMetrics(), eventBus)
		startPool(t, rts.pools[nodeID], stateStores[idx])

		require.NoError(t, err)

		rts.peerChans[nodeID] = make(chan p2p.PeerUpdate)
		pu := p2p.NewPeerUpdates(rts.peerChans[nodeID], 1)
		rts.peerUpdates[nodeID] = pu
		rts.network.Nodes[nodeID].PeerManager.Register(ctx, pu)
		rts.nodes = append(rts.nodes, rts.network.Nodes[nodeID])

		chCreator := func(ctx context.Context, chdesc *p2p.ChannelDescriptor) (p2p.Channel, error) {
			return rts.evidenceChannels[nodeID], nil
		}

		rts.reactors[nodeID] = evidence.NewReactor(
			logger,
			chCreator,
			func(ctx context.Context) *p2p.PeerUpdates { return pu },
			rts.pools[nodeID])

		require.NoError(t, rts.reactors[nodeID].Start(ctx))
		require.True(t, rts.reactors[nodeID].IsRunning())

		idx++
	}

	t.Cleanup(func() {
		for _, r := range rts.reactors {
			if r.IsRunning() {
				r.Stop()
				r.Wait()
				require.False(t, r.IsRunning())
			}
		}

	})
	t.Cleanup(leaktest.Check(t))

	return rts
}

func (rts *reactorTestSuite) start(ctx context.Context, t *testing.T) {
	rts.network.Start(ctx, t)
	require.Len(t,
		rts.network.RandomNode().PeerManager.Peers(),
		rts.numStateStores-1,
		"network does not have expected number of nodes")
}

func (rts *reactorTestSuite) waitForEvidence(t *testing.T, evList types.EvidenceList, ids ...types.NodeID) {
	t.Helper()

	fn := func(pool *evidence.Pool) {
		var (
			localEvList []types.Evidence
			size        int64
			loops       int
		)

		// wait till we have at least the amount of evidence
		// that we expect. if there's more local evidence then
		// it doesn't make sense to wait longer and a
		// different assertion should catch the resulting error
		for len(localEvList) < len(evList) {
			// each evidence should not be more than 500 bytes
			localEvList, size = pool.PendingEvidence(int64(len(evList) * 500))
			if loops == 100 {
				t.Log("current wait status:", "|",
					"local", len(localEvList), "|",
					"waitlist", len(evList), "|",
					"size", size)
			}

			loops++
		}

		// put the reaped evidence in a map so we can quickly check we got everything
		evMap := make(map[string]types.Evidence)
		for _, e := range localEvList {
			evMap[string(e.Hash())] = e
		}

		for i, expectedEv := range evList {
			gotEv := evMap[string(expectedEv.Hash())]
			require.Equalf(
				t,
				expectedEv,
				gotEv,
				"evidence for pool %d in pool does not match; got: %v, expected: %v", i, gotEv, expectedEv,
			)
		}
	}

	if len(ids) == 1 {
		// special case waiting once, just to avoid the extra
		// goroutine, in the case that this hits a timeout,
		// the stack will be clearer.
		fn(rts.pools[ids[0]])
		return
	}

	wg := sync.WaitGroup{}

	for id := range rts.pools {
		if len(ids) > 0 && !p2ptest.NodeInSlice(id, ids) {
			// if an ID list is specified, then we only
			// want to wait for those pools that are
			// specified in the list, otherwise, wait for
			// all pools.
			continue
		}

		wg.Add(1)
		go func(id types.NodeID) { defer wg.Done(); fn(rts.pools[id]) }(id)
	}
	wg.Wait()
}

func createEvidenceList(
	ctx context.Context,
	t *testing.T,
	pool *evidence.Pool,
	val types.PrivValidator,
	numEvidence int,
) types.EvidenceList {
	t.Helper()

	evList := make([]types.Evidence, numEvidence)

	for i := 0; i < numEvidence; i++ {
		ev, err := types.NewMockDuplicateVoteEvidenceWithValidator(
			ctx,
			int64(i+1),
			time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC),
			val,
			evidenceChainID,
		)
		require.NoError(t, err)
		err = pool.AddEvidence(ctx, ev)
		require.NoError(t, err,
			"adding evidence it#%d of %d to pool with height %d",
			i, numEvidence, pool.State().LastBlockHeight)
		evList[i] = ev
	}

	return evList
}

func TestReactorMultiDisconnect(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	val := types.NewMockPV()
	height := int64(numEvidence) + 10

	stateDB1 := initializeValidatorState(ctx, t, val, height)
	stateDB2 := initializeValidatorState(ctx, t, val, height)

	rts := setup(ctx, t, []sm.Store{stateDB1, stateDB2})
	primary := rts.nodes[0]
	secondary := rts.nodes[1]

	_ = createEvidenceList(ctx, t, rts.pools[primary.NodeID], val, numEvidence)

	require.Equal(t, primary.PeerManager.Status(secondary.NodeID), p2p.PeerStatusDown)

	rts.start(ctx, t)

	require.Equal(t, primary.PeerManager.Status(secondary.NodeID), p2p.PeerStatusUp)
	// Ensure "disconnecting" the secondary peer from the primary more than once
	// is handled gracefully.

	primary.PeerManager.Disconnected(ctx, secondary.NodeID)
	require.Equal(t, primary.PeerManager.Status(secondary.NodeID), p2p.PeerStatusDown)
	_, err := primary.PeerManager.TryEvictNext()
	require.NoError(t, err)
	primary.PeerManager.Disconnected(ctx, secondary.NodeID)

	require.Equal(t, primary.PeerManager.Status(secondary.NodeID), p2p.PeerStatusDown)
	require.Equal(t, secondary.PeerManager.Status(primary.NodeID), p2p.PeerStatusUp)

}

// TestReactorBroadcastEvidence creates an environment of multiple peers that
// are all at the same height. One peer, designated as a primary, gossips all
// evidence to the remaining peers.
func TestReactorBroadcastEvidence(t *testing.T) {
	numPeers := 7

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create a stateDB for all test suites (nodes)
	stateDBs := make([]sm.Store, numPeers)
	val := types.NewMockPV()

	// We need all validators saved for heights at least as high as we have
	// evidence for.
	height := int64(numEvidence) + 10
	for i := 0; i < numPeers; i++ {
		stateDBs[i] = initializeValidatorState(ctx, t, val, height)
	}

	rts := setup(ctx, t, stateDBs)

	rts.start(ctx, t)

	// Create a series of fixtures where each suite contains a reactor and
	// evidence pool. In addition, we mark a primary suite and the rest are
	// secondaries where each secondary is added as a peer via a PeerUpdate to the
	// primary. As a result, the primary will gossip all evidence to each secondary.

	primary := rts.network.RandomNode()
	secondaries := make([]*p2ptest.Node, 0, len(rts.network.NodeIDs())-1)
	secondaryIDs := make([]types.NodeID, 0, cap(secondaries))
	for id := range rts.network.Nodes {
		if id == primary.NodeID {
			continue
		}

		secondaries = append(secondaries, rts.network.Nodes[id])
		secondaryIDs = append(secondaryIDs, id)
	}

	evList := createEvidenceList(ctx, t, rts.pools[primary.NodeID], val, numEvidence)

	// Add each secondary suite (node) as a peer to the primary suite (node). This
	// will cause the primary to gossip all evidence to the secondaries.
	for _, suite := range secondaries {
		rts.peerChans[primary.NodeID] <- p2p.PeerUpdate{
			Status: p2p.PeerStatusUp,
			NodeID: suite.NodeID,
		}
	}

	// Wait till all secondary suites (reactor) received all evidence from the
	// primary suite (node).
	rts.waitForEvidence(t, evList, secondaryIDs...)

	for _, pool := range rts.pools {
		require.Equal(t, numEvidence, int(pool.Size()))
	}

}

// TestReactorSelectiveBroadcast tests a context where we have two reactors
// connected to one another but are at different heights. Reactor 1 which is
// ahead receives a list of evidence.
func TestReactorBroadcastEvidence_Lagging(t *testing.T) {
	val := types.NewMockPV()
	height1 := int64(numEvidence) + 10
	height2 := int64(numEvidence) / 2

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// stateDB1 is ahead of stateDB2, where stateDB1 has all heights (1-20) and
	// stateDB2 only has heights 1-5.
	stateDB1 := initializeValidatorState(ctx, t, val, height1)
	stateDB2 := initializeValidatorState(ctx, t, val, height2)

	rts := setup(ctx, t, []sm.Store{stateDB1, stateDB2})
	rts.start(ctx, t)

	primary := rts.nodes[0]
	secondary := rts.nodes[1]

	// Send a list of valid evidence to the first reactor's, the one that is ahead,
	// evidence pool.
	evList := createEvidenceList(ctx, t, rts.pools[primary.NodeID], val, numEvidence)

	// Add each secondary suite (node) as a peer to the primary suite (node). This
	// will cause the primary to gossip all evidence to the secondaries.
	rts.peerChans[primary.NodeID] <- p2p.PeerUpdate{
		Status: p2p.PeerStatusUp,
		NodeID: secondary.NodeID,
	}

	// only ones less than the peers height should make it through
	rts.waitForEvidence(t, evList[:height2], secondary.NodeID)

	require.Equal(t, numEvidence, int(rts.pools[primary.NodeID].Size()))
	require.Equal(t, int(height2), int(rts.pools[secondary.NodeID].Size()))
}

func TestReactorBroadcastEvidence_Pending(t *testing.T) {
	val := types.NewMockPV()
	height := int64(10)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stateDB1 := initializeValidatorState(ctx, t, val, height)
	stateDB2 := initializeValidatorState(ctx, t, val, height)

	rts := setup(ctx, t, []sm.Store{stateDB1, stateDB2})
	primary := rts.nodes[0]
	secondary := rts.nodes[1]

	evList := createEvidenceList(ctx, t, rts.pools[primary.NodeID], val, numEvidence)

	// Manually add half the evidence to the secondary which will mark them as
	// pending.
	for i := 0; i < numEvidence/2; i++ {
		err := rts.pools[secondary.NodeID].AddEvidence(ctx, evList[i])
		require.NoError(t, err)
	}

	// the secondary should have half the evidence as pending
	require.Equal(t, numEvidence/2, int(rts.pools[secondary.NodeID].Size()))

	rts.start(ctx, t)

	// The secondary reactor should have received all the evidence ignoring the
	// already pending evidence.
	rts.waitForEvidence(t, evList, secondary.NodeID)

	// check to make sure that all of the evidence has
	// propogated
	require.Len(t, rts.pools, 2)
	assert.EqualValues(t, numEvidence, rts.pools[primary.NodeID].Size(),
		"primary node should have all the evidence")
	assert.EqualValues(t, numEvidence, rts.pools[secondary.NodeID].Size(),
		"secondary nodes should have caught up")
}

func TestReactorBroadcastEvidence_Committed(t *testing.T) {
	val := types.NewMockPV()
	height := int64(10)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stateDB1 := initializeValidatorState(ctx, t, val, height)
	stateDB2 := initializeValidatorState(ctx, t, val, height)

	rts := setup(ctx, t, []sm.Store{stateDB1, stateDB2})

	primary := rts.nodes[0]
	secondary := rts.nodes[1]

	// add all evidence to the primary reactor
	evList := createEvidenceList(ctx, t, rts.pools[primary.NodeID], val, numEvidence)

	// Manually add half the evidence to the secondary which will mark them as
	// pending.
	for i := 0; i < numEvidence/2; i++ {
		err := rts.pools[secondary.NodeID].AddEvidence(ctx, evList[i])
		require.NoError(t, err)
	}

	// the secondary should have half the evidence as pending
	require.Equal(t, numEvidence/2, int(rts.pools[secondary.NodeID].Size()))

	state, err := stateDB2.Load()
	require.NoError(t, err)

	// update the secondary's pool such that all pending evidence is committed
	state.LastBlockHeight++
	rts.pools[secondary.NodeID].Update(ctx, state, evList[:numEvidence/2])

	// the secondary should have half the evidence as committed
	require.Equal(t, 0, int(rts.pools[secondary.NodeID].Size()))

	// start the network and ensure it's configured
	rts.start(ctx, t)

	// The secondary reactor should have received all the evidence ignoring the
	// already committed evidence.
	rts.waitForEvidence(t, evList[numEvidence/2:], secondary.NodeID)

	require.Len(t, rts.pools, 2)
	assert.EqualValues(t, numEvidence, rts.pools[primary.NodeID].Size(),
		"primary node should have all the evidence")
	assert.EqualValues(t, numEvidence/2, rts.pools[secondary.NodeID].Size(),
		"secondary nodes should have caught up")
}

func TestReactorBroadcastEvidence_FullyConnected(t *testing.T) {
	numPeers := 7

	// create a stateDB for all test suites (nodes)
	stateDBs := make([]sm.Store, numPeers)
	val := types.NewMockPV()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// We need all validators saved for heights at least as high as we have
	// evidence for.
	height := int64(numEvidence) + 10
	for i := 0; i < numPeers; i++ {
		stateDBs[i] = initializeValidatorState(ctx, t, val, height)
	}

	rts := setup(ctx, t, stateDBs)
	rts.start(ctx, t)

	evList := createEvidenceList(ctx, t, rts.pools[rts.network.RandomNode().NodeID], val, numEvidence)

	// every suite (reactor) connects to every other suite (reactor)
	for outerID, outerChan := range rts.peerChans {
		for innerID := range rts.peerChans {
			if outerID != innerID {
				outerChan <- p2p.PeerUpdate{
					Status: p2p.PeerStatusUp,
					NodeID: innerID,
				}
			}
		}
	}

	// wait till all suites (reactors) received all evidence from other suites (reactors)
	rts.waitForEvidence(t, evList)

	for _, pool := range rts.pools {
		require.Equal(t, numEvidence, int(pool.Size()))

		// commit state so we do not continue to repeat gossiping the same evidence
		state := pool.State()
		state.LastBlockHeight++
		pool.Update(ctx, state, evList)
	}
}

func TestEvidenceListSerialization(t *testing.T) {
	exampleVote := func(msgType byte) *types.Vote {
		var stamp, err = time.Parse(types.TimeFormat, "2017-12-25T03:00:01.234Z")
		require.NoError(t, err)

		return &types.Vote{
			Type:      tmproto.SignedMsgType(msgType),
			Height:    3,
			Round:     2,
			Timestamp: stamp,
			BlockID: types.BlockID{
				Hash: crypto.Checksum([]byte("blockID_hash")),
				PartSetHeader: types.PartSetHeader{
					Total: 1000000,
					Hash:  crypto.Checksum([]byte("blockID_part_set_header_hash")),
				},
			},
			ValidatorAddress: crypto.AddressHash([]byte("validator_address")),
			ValidatorIndex:   56789,
		}
	}

	val := &types.Validator{
		Address:     crypto.AddressHash([]byte("validator_address")),
		VotingPower: 10,
	}

	valSet := types.NewValidatorSet([]*types.Validator{val})

	dupl, err := types.NewDuplicateVoteEvidence(
		exampleVote(1),
		exampleVote(2),
		defaultEvidenceTime,
		valSet,
	)
	require.NoError(t, err)

	testCases := map[string]struct {
		evidenceList []types.Evidence
		expBytes     string
	}{
		"DuplicateVoteEvidence": {
			[]types.Evidence{dupl},
			"0a85020a82020a79080210031802224a0a208b01023386c371778ecb6368573e539afc3cc860ec3a2f614e54fe5652f4fc80122608c0843d122072db3d959635dff1bb567bedaa70573392c5159666a3f8caf11e413aac52207a2a0b08b1d381d20510809dca6f32146af1f4111082efb388211bc72c55bcd61e9ac3d538d5bb031279080110031802224a0a208b01023386c371778ecb6368573e539afc3cc860ec3a2f614e54fe5652f4fc80122608c0843d122072db3d959635dff1bb567bedaa70573392c5159666a3f8caf11e413aac52207a2a0b08b1d381d20510809dca6f32146af1f4111082efb388211bc72c55bcd61e9ac3d538d5bb03180a200a2a060880dbaae105",
		},
	}

	for name, tc := range testCases {
		tc := tc

		t.Run(name, func(t *testing.T) {
			protoEv := make([]tmproto.Evidence, len(tc.evidenceList))
			for i := 0; i < len(tc.evidenceList); i++ {
				ev, err := types.EvidenceToProto(tc.evidenceList[i])
				require.NoError(t, err)
				protoEv[i] = *ev
			}

			epl := tmproto.EvidenceList{
				Evidence: protoEv,
			}

			bz, err := epl.Marshal()
			require.NoError(t, err)

			require.Equal(t, tc.expBytes, hex.EncodeToString(bz))
		})
	}
}
