package evidence

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/go-kit/kit/log/term"
	"github.com/stretchr/testify/assert"

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/crypto/secp256k1"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/types"
	dbm "github.com/tendermint/tm-db"
)

// evidenceLogger is a TestingLogger which uses a different
// color for each validator ("validator" key must exist).
func evidenceLogger() log.Logger {
	return log.TestingLoggerWithColorFn(func(keyvals ...interface{}) term.FgBgColor {
		for i := 0; i < len(keyvals)-1; i += 2 {
			if keyvals[i] == "validator" {
				return term.FgBgColor{Fg: term.Color(uint8(keyvals[i+1].(int) + 1))}
			}
		}
		return term.FgBgColor{}
	})
}

// connect N evidence reactors through N switches
func makeAndConnectEvidenceReactors(config *cfg.Config, stateDBs []dbm.DB) []*EvidenceReactor {
	N := len(stateDBs)
	reactors := make([]*EvidenceReactor, N)
	logger := evidenceLogger()
	for i := 0; i < N; i++ {

		evidenceDB := dbm.NewMemDB()
		pool := NewEvidencePool(stateDBs[i], evidenceDB)
		reactors[i] = NewEvidenceReactor(pool)
		reactors[i].SetLogger(logger.With("validator", i))
	}

	p2p.MakeConnectedSwitches(config.P2P, N, func(i int, s *p2p.Switch) *p2p.Switch {
		s.AddReactor("EVIDENCE", reactors[i])
		return s

	}, p2p.Connect2Switches)
	return reactors
}

// wait for all evidence on all reactors
func waitForEvidence(t *testing.T, evs types.EvidenceList, reactors []*EvidenceReactor) {
	// wait for the evidence in all evpools
	wg := new(sync.WaitGroup)
	for i := 0; i < len(reactors); i++ {
		wg.Add(1)
		go _waitForEvidence(t, wg, evs, i, reactors)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	timer := time.After(TIMEOUT)
	select {
	case <-timer:
		t.Fatal("Timed out waiting for evidence")
	case <-done:
	}
}

// wait for all evidence on a single evpool
func _waitForEvidence(t *testing.T, wg *sync.WaitGroup, evs types.EvidenceList, reactorIdx int, reactors []*EvidenceReactor) {

	evpool := reactors[reactorIdx].evpool
	for len(evpool.PendingEvidence(-1)) != len(evs) {
		time.Sleep(time.Millisecond * 100)
	}

	reapedEv := evpool.PendingEvidence(-1)
	// put the reaped evidence in a map so we can quickly check we got everything
	evMap := make(map[string]types.Evidence)
	for _, e := range reapedEv {
		evMap[string(e.Hash())] = e
	}
	for i, expectedEv := range evs {
		gotEv := evMap[string(expectedEv.Hash())]
		assert.Equal(t, expectedEv, gotEv,
			fmt.Sprintf("evidence at index %d on reactor %d don't match: %v vs %v",
				i, reactorIdx, expectedEv, gotEv))
	}

	wg.Done()
}

func sendEvidence(t *testing.T, evpool *EvidencePool, valAddr []byte, n int) types.EvidenceList {
	evList := make([]types.Evidence, n)
	for i := 0; i < n; i++ {
		ev := types.NewMockGoodEvidence(int64(i+1), 0, valAddr)
		err := evpool.AddEvidence(ev)
		assert.Nil(t, err)
		evList[i] = ev
	}
	return evList
}

var (
	NUM_EVIDENCE = 10
	TIMEOUT      = 120 * time.Second // ridiculously high because CircleCI is slow
)

func TestReactorBroadcastEvidence(t *testing.T) {
	config := cfg.TestConfig()
	N := 7

	// create statedb for everyone
	stateDBs := make([]dbm.DB, N)
	valAddr := []byte("myval")
	// we need validators saved for heights at least as high as we have evidence for
	height := int64(NUM_EVIDENCE) + 10
	for i := 0; i < N; i++ {
		stateDBs[i] = initializeValidatorState(valAddr, height)
	}

	// make reactors from statedb
	reactors := makeAndConnectEvidenceReactors(config, stateDBs)

	// set the peer height on each reactor
	for _, r := range reactors {
		for _, peer := range r.Switch.Peers().List() {
			ps := peerState{height}
			peer.Set(types.PeerStateKey, ps)
		}
	}

	// send a bunch of valid evidence to the first reactor's evpool
	// and wait for them all to be received in the others
	evList := sendEvidence(t, reactors[0].evpool, valAddr, NUM_EVIDENCE)
	waitForEvidence(t, evList, reactors)
}

type peerState struct {
	height int64
}

func (ps peerState) GetHeight() int64 {
	return ps.height
}

func TestReactorSelectiveBroadcast(t *testing.T) {
	config := cfg.TestConfig()

	valAddr := []byte("myval")
	height1 := int64(NUM_EVIDENCE) + 10
	height2 := int64(NUM_EVIDENCE) / 2

	// DB1 is ahead of DB2
	stateDB1 := initializeValidatorState(valAddr, height1)
	stateDB2 := initializeValidatorState(valAddr, height2)

	// make reactors from statedb
	reactors := makeAndConnectEvidenceReactors(config, []dbm.DB{stateDB1, stateDB2})

	// set the peer height on each reactor
	for _, r := range reactors {
		for _, peer := range r.Switch.Peers().List() {
			ps := peerState{height1}
			peer.Set(types.PeerStateKey, ps)
		}
	}

	// update the first reactor peer's height to be very small
	peer := reactors[0].Switch.Peers().List()[0]
	ps := peerState{height2}
	peer.Set(types.PeerStateKey, ps)

	// send a bunch of valid evidence to the first reactor's evpool
	evList := sendEvidence(t, reactors[0].evpool, valAddr, NUM_EVIDENCE)

	// only ones less than the peers height should make it through
	waitForEvidence(t, evList[:NUM_EVIDENCE/2], reactors[1:2])

	// peers should still be connected
	peers := reactors[1].Switch.Peers().List()
	assert.Equal(t, 1, len(peers))
}
func TestEvidenceListMessageValidationBasic(t *testing.T) {

	testCases := []struct {
		testName          string
		malleateEvListMsg func(*EvidenceListMessage)
		expectErr         bool
	}{
		{"Good EvidenceListMessage", func(evList *EvidenceListMessage) {}, false},
		{"Invalid EvidenceListMessage", func(evList *EvidenceListMessage) {
			evList.Evidence = append(evList.Evidence,
				&types.DuplicateVoteEvidence{PubKey: secp256k1.GenPrivKey().PubKey()})
		}, true},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			evListMsg := &EvidenceListMessage{}
			n := 3
			valAddr := []byte("myval")
			evListMsg.Evidence = make([]types.Evidence, n)
			for i := 0; i < n; i++ {
				evListMsg.Evidence[i] = types.NewMockGoodEvidence(int64(i+1), 0, valAddr)
			}
			tc.malleateEvListMsg(evListMsg)
			assert.Equal(t, tc.expectErr, evListMsg.ValidateBasic() != nil, "Validate Basic had an unexpected result")
		})
	}
}
