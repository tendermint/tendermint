package consensus

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/internal/eventbus"
	"github.com/tendermint/tendermint/internal/p2p"
	"github.com/tendermint/tendermint/libs/bytes"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	tmtime "github.com/tendermint/tendermint/libs/time"
	tmcons "github.com/tendermint/tendermint/proto/tendermint/consensus"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

func TestReactorInvalidPrecommit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	config := configSetup(t)

	n := 4
	states, cleanup := makeConsensusState(ctx, t,
		config, n, "consensus_reactor_test",
		newMockTickerFunc(true))
	t.Cleanup(cleanup)

	for i := 0; i < 4; i++ {
		ticker := NewTimeoutTicker(states[i].logger)
		states[i].SetTimeoutTicker(ticker)
	}

	rts := setup(ctx, t, n, states, 100) // buffer must be large enough to not deadlock

	for _, reactor := range rts.reactors {
		state := reactor.state.GetState()
		reactor.SwitchToConsensus(ctx, state, false)
	}

	// this val sends a random precommit at each height
	node := rts.network.RandomNode()

	byzState := rts.states[node.NodeID]
	byzReactor := rts.reactors[node.NodeID]

	calledDoPrevote := false
	// Update the doPrevote function to just send a valid precommit for a random
	// block and otherwise disable the priv validator.
	byzState.mtx.Lock()
	privVal := byzState.privValidator
	byzState.doPrevote = func(ctx context.Context, height int64, round int32) {
		invalidDoPrevoteFunc(ctx, t, height, round, byzState, byzReactor, privVal)
		calledDoPrevote = true
	}
	byzState.mtx.Unlock()

	// wait for a bunch of blocks
	//
	// TODO: Make this tighter by ensuring the halt happens by block 2.
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		for _, sub := range rts.subs {
			wg.Add(1)

			go func(s eventbus.Subscription) {
				defer wg.Done()
				_, err := s.Next(ctx)
				if !assert.NoError(t, err) {
					cancel() // cancel other subscribers on failure
				}
			}(sub)
		}
	}

	wg.Wait()
	if !calledDoPrevote {
		t.Fatal("test failed to run core logic")
	}
}

func invalidDoPrevoteFunc(
	ctx context.Context,
	t *testing.T,
	height int64,
	round int32,
	cs *State,
	r *Reactor,
	pv types.PrivValidator,
) {
	// routine to:
	// - precommit for a random block
	// - send precommit to all peers
	// - disable privValidator (so we don't do normal precommits)
	go func() {
		cs.mtx.Lock()
		cs.privValidator = pv

		pubKey, err := cs.privValidator.GetPubKey(ctx)
		require.NoError(t, err)

		addr := pubKey.Address()
		valIndex, _ := cs.Validators.GetByAddress(addr)

		// precommit a random block
		blockHash := bytes.HexBytes(tmrand.Bytes(32))
		precommit := &types.Vote{
			ValidatorAddress: addr,
			ValidatorIndex:   valIndex,
			Height:           cs.Height,
			Round:            cs.Round,
			Timestamp:        tmtime.Now(),
			Type:             tmproto.PrecommitType,
			BlockID: types.BlockID{
				Hash:          blockHash,
				PartSetHeader: types.PartSetHeader{Total: 1, Hash: tmrand.Bytes(32)}},
		}

		p := precommit.ToProto()
		err = cs.privValidator.SignVote(ctx, cs.state.ChainID, p)
		require.NoError(t, err)

		precommit.Signature = p.Signature
		cs.privValidator = nil // disable priv val so we don't do normal votes
		cs.mtx.Unlock()

		count := 0
		for _, ps := range r.peers {
			count++
			err := r.voteCh.Send(ctx, p2p.Envelope{
				To: ps.peerID,
				Message: &tmcons.Vote{
					Vote: precommit.ToProto(),
				},
			})
			// we want to have sent some of these votes,
			// but if the test completes without erroring
			// and we get here, we shouldn't error
			if errors.Is(err, context.Canceled) && count > 1 {
				break
			}
			require.NoError(t, err)
		}
	}()
}
