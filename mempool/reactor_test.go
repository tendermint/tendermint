// Copyright 2017 Tendermint. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mempool

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

// mempoolLogger is a TestingLogger which uses a different
// color for each validator ("validator" key must exist).
func mempoolLogger() log.Logger {
	return log.TestingLoggerWithColorFn(func(keyvals ...interface{}) term.FgBgColor {
		for i := 0; i < len(keyvals)-1; i += 2 {
			if keyvals[i] == "validator" {
				return term.FgBgColor{Fg: term.Color(uint8(keyvals[i+1].(int) + 1))}
			}
		}
		return term.FgBgColor{}
	})
}

// connect N mempool reactors through N switches
func makeAndConnectMempoolReactors(config *cfg.Config, N int) []*MempoolReactor {
	reactors := make([]*MempoolReactor, N)
	logger := mempoolLogger()
	for i := 0; i < N; i++ {
		app := dummy.NewDummyApplication()
		cc := proxy.NewLocalClientCreator(app)
		mempool := newMempoolWithApp(cc)

		reactors[i] = NewMempoolReactor(config.Mempool, mempool) // so we dont start the consensus states
		reactors[i].SetLogger(logger.With("validator", i))
	}

	p2p.MakeConnectedSwitches(config.P2P, N, func(i int, s *p2p.Switch) *p2p.Switch {
		s.AddReactor("MEMPOOL", reactors[i])
		return s

	}, p2p.Connect2Switches)
	return reactors
}

// wait for all txs on all reactors
func waitForTxs(t *testing.T, txs types.Txs, reactors []*MempoolReactor) {
	// wait for the txs in all mempools
	wg := new(sync.WaitGroup)
	for i := 0; i < len(reactors); i++ {
		wg.Add(1)
		go _waitForTxs(t, wg, txs, i, reactors)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	timer := time.After(TIMEOUT)
	select {
	case <-timer:
		t.Fatal("Timed out waiting for txs")
	case <-done:
	}
}

// wait for all txs on a single mempool
func _waitForTxs(t *testing.T, wg *sync.WaitGroup, txs types.Txs, reactorIdx int, reactors []*MempoolReactor) {

	mempool := reactors[reactorIdx].Mempool
	for mempool.Size() != len(txs) {
		time.Sleep(time.Second)
	}

	reapedTxs := mempool.Reap(len(txs))
	for i, tx := range txs {
		assert.Equal(t, tx, reapedTxs[i], fmt.Sprintf("txs at index %d on reactor %d don't match: %v vs %v", i, reactorIdx, tx, reapedTxs[i]))
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
	reactors := makeAndConnectMempoolReactors(config, N)

	// send a bunch of txs to the first reactor's mempool
	// and wait for them all to be received in the others
	txs := checkTxs(t, reactors[0].Mempool, NUM_TXS)
	waitForTxs(t, txs, reactors)
}
