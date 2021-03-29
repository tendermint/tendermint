package evidence_test

import (
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
	"github.com/tendermint/tendermint/crypto/tmhash"
	"github.com/tendermint/tendermint/evidence"
	"github.com/tendermint/tendermint/evidence/mocks"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/p2p/p2ptest"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

var (
	numEvidence = 10

	rng = rand.New(rand.NewSource(time.Now().UnixNano()))
)

type reactorTestSuite struct {
	network          *p2ptest.Network
	logger           log.Logger
	reactors         map[p2p.NodeID]*evidence.Reactor
	pools            map[p2p.NodeID]*evidence.Pool
	evidenceChannels map[p2p.NodeID]*p2p.Channel
	peerUpdates      map[p2p.NodeID]*p2p.PeerUpdates
	peerChans        map[p2p.NodeID]chan p2p.PeerUpdate
	nodes            []*p2ptest.Node
	numStateStores   int
}

func setup(t *testing.T, stateStores []sm.Store, chBuf uint) *reactorTestSuite {
	t.Helper()

	pID := make([]byte, 16)
	_, err := rng.Read(pID)
	require.NoError(t, err)

	numStateStores := len(stateStores)
	rts := &reactorTestSuite{
		numStateStores: numStateStores,
		logger:         log.TestingLogger().With("testCase", t.Name()),
		network:        p2ptest.MakeNetwork(t, p2ptest.NetworkOptions{NumNodes: numStateStores}),
		reactors:       make(map[p2p.NodeID]*evidence.Reactor, numStateStores),
		pools:          make(map[p2p.NodeID]*evidence.Pool, numStateStores),
		peerUpdates:    make(map[p2p.NodeID]*p2p.PeerUpdates, numStateStores),
		peerChans:      make(map[p2p.NodeID]chan p2p.PeerUpdate, numStateStores),
	}

	rts.evidenceChannels = rts.network.MakeChannelsNoCleanup(t,
		evidence.EvidenceChannel,
		new(tmproto.EvidenceList),
		int(chBuf))
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
		rts.pools[nodeID], err = evidence.NewPool(logger, evidenceDB, stateStores[idx], blockStore)

		require.NoError(t, err)

		rts.peerChans[nodeID] = make(chan p2p.PeerUpdate)
		rts.peerUpdates[nodeID] = p2p.NewPeerUpdates(rts.peerChans[nodeID], 1)
		rts.network.Nodes[nodeID].PeerManager.Register(rts.peerUpdates[nodeID])
		rts.nodes = append(rts.nodes, rts.network.Nodes[nodeID])

		rts.reactors[nodeID] = evidence.NewReactor(logger,
			rts.evidenceChannels[nodeID],
			rts.peerUpdates[nodeID],
			rts.pools[nodeID])

		require.NoError(t, rts.reactors[nodeID].Start())
		require.True(t, rts.reactors[nodeID].IsRunning())

		idx++
	}

	t.Cleanup(func() {
		for _, r := range rts.reactors {
			if r.IsRunning() {
				require.NoError(t, r.Stop())
				require.False(t, r.IsRunning())
			}
		}

		leaktest.Check(t)
	})

	return rts
}

func (rts *reactorTestSuite) start(t *testing.T) {
	rts.network.Start(t)
	require.Len(t,
		rts.network.RandomNode().PeerManager.Peers(),
		rts.numStateStores-1,
		"network does not have expected number of nodes")
}

func (rts *reactorTestSuite) waitForEvidence(t *testing.T, evList types.EvidenceList, ids ...p2p.NodeID) {
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
		go func(id p2p.NodeID) { defer wg.Done(); fn(rts.pools[id]) }(id)
	}
	wg.Wait()
}

func (rts *reactorTestSuite) assertEvidenceChannelsEmpty(t *testing.T) {
	t.Helper()

	for id, r := range rts.reactors {
		require.NoError(t, r.Stop(), "stopping reactor #%s", id)
		r.Wait()
		require.False(t, r.IsRunning(), "reactor #%d did not stop", id)

	}

	for id, ech := range rts.evidenceChannels {
		require.Empty(t, ech.Out, "checking channel #%q", id)
	}
}

func createEvidenceList(
	t *testing.T,
	pool *evidence.Pool,
	val types.PrivValidator,
	numEvidence int,
) types.EvidenceList {
	t.Helper()

	evList := make([]types.Evidence, numEvidence)

	for i := 0; i < numEvidence; i++ {
		ev := types.NewMockDuplicateVoteEvidenceWithValidator(
			int64(i+1),
			time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC),
			val,
			evidenceChainID,
		)

		require.NoError(t, pool.AddEvidence(ev),
			"adding evidence it#%d of %d to pool with height %d",
			i, numEvidence, pool.State().LastBlockHeight)
		evList[i] = ev
	}

	return evList
}

func TestReactorMultiDisconnect(t *testing.T) {
	val := types.NewMockPV()
	height := int64(numEvidence) + 10

	stateDB1 := initializeValidatorState(t, val, height)
	stateDB2 := initializeValidatorState(t, val, height)

	rts := setup(t, []sm.Store{stateDB1, stateDB2}, 20)
	primary := rts.nodes[0]
	secondary := rts.nodes[1]

	_ = createEvidenceList(t, rts.pools[primary.NodeID], val, numEvidence)

	require.Equal(t, primary.PeerManager.Status(secondary.NodeID), p2p.PeerStatusDown)

	rts.start(t)

	require.Equal(t, primary.PeerManager.Status(secondary.NodeID), p2p.PeerStatusUp)
	// Ensure "disconnecting" the secondary peer from the primary more than once
	// is handled gracefully.

	require.NoError(t, primary.PeerManager.Disconnected(secondary.NodeID))
	require.Equal(t, primary.PeerManager.Status(secondary.NodeID), p2p.PeerStatusDown)
	_, err := primary.PeerManager.TryEvictNext()
	require.NoError(t, err)
	require.NoError(t, primary.PeerManager.Disconnected(secondary.NodeID))

	require.Equal(t, primary.PeerManager.Status(secondary.NodeID), p2p.PeerStatusDown)
	require.Equal(t, secondary.PeerManager.Status(primary.NodeID), p2p.PeerStatusUp)

}

// TestReactorBroadcastEvidence creates an environment of multiple peers that
// are all at the same height. One peer, designated as a primary, gossips all
// evidence to the remaining peers.
func TestReactorBroadcastEvidence(t *testing.T) {
	numPeers := 7

	// create a stateDB for all test suites (nodes)
	stateDBs := make([]sm.Store, numPeers)
	val := types.NewMockPV()

	// We need all validators saved for heights at least as high as we have
	// evidence for.
	height := int64(numEvidence) + 10
	for i := 0; i < numPeers; i++ {
		stateDBs[i] = initializeValidatorState(t, val, height)
	}

	rts := setup(t, stateDBs, 0)
	rts.start(t)

	// Create a series of fixtures where each suite contains a reactor and
	// evidence pool. In addition, we mark a primary suite and the rest are
	// secondaries where each secondary is added as a peer via a PeerUpdate to the
	// primary. As a result, the primary will gossip all evidence to each secondary.
	primary := rts.network.RandomNode()
	secondaries := make([]*p2ptest.Node, 0, len(rts.network.NodeIDs())-1)
	secondaryIDs := make([]p2p.NodeID, 0, cap(secondaries))
	for id := range rts.network.Nodes {
		if id == primary.NodeID {
			continue
		}

		secondaries = append(secondaries, rts.network.Nodes[id])
		secondaryIDs = append(secondaryIDs, id)
	}

	evList := createEvidenceList(t, rts.pools[primary.NodeID], val, numEvidence)

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

	rts.assertEvidenceChannelsEmpty(t)
}

// TestReactorSelectiveBroadcast tests a context where we have two reactors
// connected to one another but are at different heights. Reactor 1 which is
// ahead receives a list of evidence.
func TestReactorBroadcastEvidence_Lagging(t *testing.T) {
	val := types.NewMockPV()
	height1 := int64(numEvidence) + 10
	height2 := int64(numEvidence) / 2

	// stateDB1 is ahead of stateDB2, where stateDB1 has all heights (1-20) and
	// stateDB2 only has heights 1-5.
	stateDB1 := initializeValidatorState(t, val, height1)
	stateDB2 := initializeValidatorState(t, val, height2)

	rts := setup(t, []sm.Store{stateDB1, stateDB2}, 100)
	rts.start(t)

	primary := rts.nodes[0]
	secondary := rts.nodes[1]

	// Send a list of valid evidence to the first reactor's, the one that is ahead,
	// evidence pool.
	evList := createEvidenceList(t, rts.pools[primary.NodeID], val, numEvidence)

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

	rts.assertEvidenceChannelsEmpty(t)
}

func TestReactorBroadcastEvidence_Pending(t *testing.T) {
	val := types.NewMockPV()
	height := int64(10)

	stateDB1 := initializeValidatorState(t, val, height)
	stateDB2 := initializeValidatorState(t, val, height)

	rts := setup(t, []sm.Store{stateDB1, stateDB2}, 100)
	primary := rts.nodes[0]
	secondary := rts.nodes[1]

	evList := createEvidenceList(t, rts.pools[primary.NodeID], val, numEvidence)

	// Manually add half the evidence to the secondary which will mark them as
	// pending.
	for i := 0; i < numEvidence/2; i++ {
		require.NoError(t, rts.pools[secondary.NodeID].AddEvidence(evList[i]))
	}

	// the secondary should have half the evidence as pending
	require.Equal(t, numEvidence/2, int(rts.pools[secondary.NodeID].Size()))

	rts.start(t)

	// The secondary reactor should have received all the evidence ignoring the
	// already pending evidence.
	rts.waitForEvidence(t, evList, secondary.NodeID)

	// check to make sure that all of the evidence has
	// propogated
	require.Len(t, rts.pools, 2)
	assert.EqualValues(t, numEvidence, rts.pools[primary.NodeID].Size(),
		"primary node should have all the evidence")
	if assert.EqualValues(t, numEvidence, rts.pools[secondary.NodeID].Size(),
		"secondary nodes should have caught up") {

		rts.assertEvidenceChannelsEmpty(t)
	}
}

func TestReactorBroadcastEvidence_Committed(t *testing.T) {
	val := types.NewMockPV()
	height := int64(10)

	stateDB1 := initializeValidatorState(t, val, height)
	stateDB2 := initializeValidatorState(t, val, height)

	rts := setup(t, []sm.Store{stateDB1, stateDB2}, 0)

	primary := rts.nodes[0]
	secondary := rts.nodes[1]

	// add all evidence to the primary reactor
	evList := createEvidenceList(t, rts.pools[primary.NodeID], val, numEvidence)

	// Manually add half the evidence to the secondary which will mark them as
	// pending.
	for i := 0; i < numEvidence/2; i++ {
		require.NoError(t, rts.pools[secondary.NodeID].AddEvidence(evList[i]))
	}

	// the secondary should have half the evidence as pending
	require.Equal(t, numEvidence/2, int(rts.pools[secondary.NodeID].Size()))

	state, err := stateDB2.Load()
	require.NoError(t, err)

	// update the secondary's pool such that all pending evidence is committed
	state.LastBlockHeight++
	rts.pools[secondary.NodeID].Update(state, evList[:numEvidence/2])

	// the secondary should have half the evidence as committed
	require.Equal(t, 0, int(rts.pools[secondary.NodeID].Size()))

	// start the network and ensure it's configured
	rts.start(t)

	// without the following sleep the test consistently fails;
	// likely because the sleep forces a context switch that lets
	// the router process other operations.
	time.Sleep(2 * time.Millisecond)

	// The secondary reactor should have received all the evidence ignoring the
	// already committed evidence.
	rts.waitForEvidence(t, evList[numEvidence/2:], secondary.NodeID)

	require.Len(t, rts.pools, 2)
	assert.EqualValues(t, numEvidence, rts.pools[primary.NodeID].Size(),
		"primary node should have all the evidence")
	if assert.EqualValues(t, numEvidence/2, rts.pools[secondary.NodeID].Size(),
		"secondary nodes should have caught up") {

		rts.assertEvidenceChannelsEmpty(t)
	}
}

func TestReactorBroadcastEvidence_FullyConnected(t *testing.T) {
	numPeers := 7

	// create a stateDB for all test suites (nodes)
	stateDBs := make([]sm.Store, numPeers)
	val := types.NewMockPV()

	// We need all validators saved for heights at least as high as we have
	// evidence for.
	height := int64(numEvidence) + 10
	for i := 0; i < numPeers; i++ {
		stateDBs[i] = initializeValidatorState(t, val, height)
	}

	rts := setup(t, stateDBs, 0)
	rts.start(t)

	evList := createEvidenceList(t, rts.pools[rts.network.RandomNode().NodeID], val, numEvidence)

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
		pool.Update(state, evList)
	}
}

// nolint:lll
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
				Hash: tmhash.Sum([]byte("blockID_hash")),
				PartSetHeader: types.PartSetHeader{
					Total: 1000000,
					Hash:  tmhash.Sum([]byte("blockID_part_set_header_hash")),
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

	dupl := types.NewDuplicateVoteEvidence(
		exampleVote(1),
		exampleVote(2),
		defaultEvidenceTime,
		valSet,
	)

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
