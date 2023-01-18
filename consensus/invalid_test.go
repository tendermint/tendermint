package consensus

import (
	"testing"

	"github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/libs/log"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/p2p"
	tmcons "github.com/tendermint/tendermint/proto/tendermint/consensus"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

//----------------------------------------------
// byzantine failures

// one byz val sends a precommit for a random block at each height
// Ensure a testnet makes blocks
func TestReactorInvalidPrecommit(t *testing.T) {
	N := 4
	css, cleanup := randConsensusNet(N, "consensus_reactor_test", newMockTickerFunc(true), newCounter)
	defer cleanup()

	for i := 0; i < 4; i++ {
		ticker := NewTimeoutTicker()
		ticker.SetLogger(css[i].Logger)
		css[i].SetTimeoutTicker(ticker)

	}

	reactors, blocksSubs, eventBuses := startConsensusNet(t, css, N)

	// this val sends a random precommit at each height
	byzValIdx := 0
	byzVal := css[byzValIdx]
	byzR := reactors[byzValIdx]

	// update the doPrevote function to just send a valid precommit for a random block
	// and otherwise disable the priv validator
	byzVal.mtx.Lock()
	pv := byzVal.privValidator
	byzVal.doPrevote = func(height int64, round int32) {
		invalidDoPrevoteFunc(t, height, round, byzVal, byzR.Switch, pv)
	}
	byzVal.mtx.Unlock()
	defer stopConsensusNet(log.TestingLogger(), reactors, eventBuses)

	// wait for a bunch of blocks
	// TODO: make this tighter by ensuring the halt happens by block 2
	for i := 0; i < 10; i++ {
		timeoutWaitGroup(t, N, func(j int) {
			<-blocksSubs[j].Out()
		}, css)
	}
}

func invalidDoPrevoteFunc(t *testing.T, height int64, round int32, cs *State, sw *p2p.Switch, pv types.PrivValidator) {
	// routine to:
	// - precommit for a random block
	// - send precommit to all peers
	// - disable privValidator (so we don't do normal precommits)
	go func() {
		cs.mtx.Lock()
		cs.privValidator = pv
		pubKey, err := cs.privValidator.GetPubKey()
		if err != nil {
			panic(err)
		}
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
		err = cs.privValidator.SignVote(cs.state.ChainID, p)
		if err != nil {
			t.Error(err)
		}
		precommit.Signature = p.Signature
		cs.privValidator = nil // disable priv val so we don't do normal votes
		cs.mtx.Unlock()

		peers := sw.Peers().List()
		for _, peer := range peers {
			cs.Logger.Info("Sending bad vote", "block", blockHash, "peer", peer)
			p2p.SendEnvelopeShim(peer, p2p.Envelope{ //nolint: staticcheck
				Message:   &tmcons.Vote{Vote: precommit.ToProto()},
				ChannelID: VoteChannel,
			}, cs.Logger)
		}
	}()
}
