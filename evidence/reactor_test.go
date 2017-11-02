package evpool

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/go-kit/kit/log/term"

	"github.com/tendermint/abci/example/dummy"
	"github.com/tendermint/tmlibs/log"

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/proxy"
	"github.com/tendermint/tendermint/types"
)

// evpoolLogger is a TestingLogger which uses a different
// color for each validator ("validator" key must exist).
func evpoolLogger() log.Logger {
	return log.TestingLoggerWithColorFn(func(keyvals ...interface{}) term.FgBgColor {
		for i := 0; i < len(keyvals)-1; i += 2 {
			if keyvals[i] == "validator" {
				return term.FgBgColor{Fg: term.Color(uint8(keyvals[i+1].(int) + 1))}
			}
		}
		return term.FgBgColor{}
	})
}

// connect N evpool reactors through N switches
func makeAndConnectEvidencePoolReactors(config *cfg.Config, N int) []*EvidencePoolReactor {
	reactors := make([]*EvidencePoolReactor, N)
	logger := evpoolLogger()
	for i := 0; i < N; i++ {
		app := dummy.NewDummyApplication()
		cc := proxy.NewLocalClientCreator(app)
		evpool := newEvidencePoolWithApp(cc)

		reactors[i] = NewEvidencePoolReactor(config.EvidencePool, evpool) // so we dont start the consensus states
		reactors[i].SetLogger(logger.With("validator", i))
	}

	p2p.MakeConnectedSwitches(config.P2P, N, func(i int, s *p2p.Switch) *p2p.Switch {
		s.AddReactor("MEMPOOL", reactors[i])
		return s

	}, p2p.Connect2Switches)
	return reactors
}

// wait for all evidences on all reactors
func waitForTxs(t *testing.T, evidences types.Txs, reactors []*EvidencePoolReactor) {
	// wait for the evidences in all evpools
	wg := new(sync.WaitGroup)
	for i := 0; i < len(reactors); i++ {
		wg.Add(1)
		go _waitForTxs(t, wg, evidences, i, reactors)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	timer := time.After(TIMEOUT)
	select {
	case <-timer:
		t.Fatal("Timed out waiting for evidences")
	case <-done:
	}
}

// wait for all evidences on a single evpool
func _waitForTxs(t *testing.T, wg *sync.WaitGroup, evidences types.Txs, reactorIdx int, reactors []*EvidencePoolReactor) {

	evpool := reactors[reactorIdx].EvidencePool
	for evpool.Size() != len(evidences) {
		time.Sleep(time.Second)
	}

	reapedTxs := evpool.Reap(len(evidences))
	for i, evidence := range evidences {
		assert.Equal(t, evidence, reapedTxs[i], fmt.Sprintf("evidences at index %d on reactor %d don't match: %v vs %v", i, reactorIdx, evidence, reapedTxs[i]))
	}
	wg.Done()
}

var (
	NUM_TXS = 1000
	TIMEOUT = 120 * time.Second // ridiculously high because CircleCI is slow
)

func TestReactorBroadcastTxMessage(t *testing.T) {
	config := cfg.TestConfig()
	N := 4
	reactors := makeAndConnectEvidencePoolReactors(config, N)

	// send a bunch of evidences to the first reactor's evpool
	// and wait for them all to be received in the others
	evidences := checkTxs(t, reactors[0].EvidencePool, NUM_TXS)
	waitForTxs(t, evidences, reactors)
}
