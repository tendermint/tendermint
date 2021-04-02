package consensus

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/bytes"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/p2p"
	tmcons "github.com/tendermint/tendermint/proto/tendermint/consensus"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

func TestReactorInvalidPrecommit(t *testing.T) {
	config := configSetup(t)

	n := 4
	states, cleanup := randConsensusState(config, n, "consensus_reactor_test", newMockTickerFunc(true), newCounter)
	t.Cleanup(cleanup)

	for i := 0; i < 4; i++ {
		ticker := NewTimeoutTicker()
		ticker.SetLogger(states[i].Logger)
		states[i].SetTimeoutTicker(ticker)
	}

	rts := setup(t, n, states, 100) // buffer must be large enough to not deadlock

	for _, reactor := range rts.reactors {
		state := reactor.state.GetState()
		reactor.SwitchToConsensus(state, false)
	}

	// this val sends a random precommit at each height
	node := rts.network.RandomNode()

	byzState := rts.states[node.NodeID]
	byzReactor := rts.reactors[node.NodeID]

	// Update the doPrevote function to just send a valid precommit for a random
	// block and otherwise disable the priv validator.
	byzState.mtx.Lock()
	privVal := byzState.privValidator
	byzState.doPrevote = func(height int64, round int32) {
		invalidDoPrevoteFunc(t, height, round, byzState, byzReactor, privVal)
	}
	byzState.mtx.Unlock()

	// wait for a bunch of blocks
	//
	// TODO: Make this tighter by ensuring the halt happens by block 2.
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		for _, sub := range rts.subs {
			wg.Add(1)

			go func(s types.Subscription) {
				<-s.Out()
				wg.Done()
			}(sub)
		}
	}

	wg.Wait()
}

func invalidDoPrevoteFunc(t *testing.T, height int64, round int32, cs *State, r *Reactor, pv types.PrivValidator) {
	// routine to:
	// - precommit for a random block
	// - send precommit to all peers
	// - disable privValidator (so we don't do normal precommits)
	go func() {
		cs.mtx.Lock()
		cs.privValidator = pv

		pubKey, err := cs.privValidator.GetPubKey(context.Background())
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
			Timestamp:        cs.voteTime(),
			Type:             tmproto.PrecommitType,
			BlockID: types.BlockID{
				Hash:          blockHash,
				PartSetHeader: types.PartSetHeader{Total: 1, Hash: tmrand.Bytes(32)}},
		}

		p := precommit.ToProto()
		err = cs.privValidator.SignVote(context.Background(), cs.state.ChainID, p)
		require.NoError(t, err)

		precommit.Signature = p.Signature
		cs.privValidator = nil // disable priv val so we don't do normal votes
		cs.mtx.Unlock()

		for _, ps := range r.peers {
			cs.Logger.Info("sending bad vote", "block", blockHash, "peer", ps.peerID)

			r.voteCh.Out <- p2p.Envelope{
				To: ps.peerID,
				Message: &tmcons.Vote{
					Vote: precommit.ToProto(),
				},
			}
		}
	}()
}
