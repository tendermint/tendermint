package consensus

import (
	"testing"

	"github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/libs/log"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/p2p"
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
	byzVal.doPrevote = func(height int64, round int32, allowOldBlocks bool) {
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
		proTxHash, err := cs.privValidator.GetProTxHash()
		if err != nil {
			panic(err)
		}
		valIndex, _ := cs.Validators.GetByProTxHash(proTxHash)

		// precommit a random block
		blockHash := bytes.HexBytes(tmrand.Bytes(32))
		lastAppHash := bytes.HexBytes(tmrand.Bytes(32))
		// we want to see both errors, so send the correct state id half the time
		if tmrand.Bool() == true {
			lastAppHash = cs.state.AppHash
		}
		precommit := &types.Vote{
			ValidatorProTxHash: proTxHash,
			ValidatorIndex:     valIndex,
			Height:             cs.Height,
			Round:              cs.Round,
			Type:               tmproto.PrecommitType,
			BlockID: types.BlockID{
				Hash:          blockHash,
				PartSetHeader: types.PartSetHeader{Total: 1, Hash: tmrand.Bytes(32)}},
			StateID: types.StateID{
				LastAppHash: lastAppHash,
			},
		}
		p := precommit.ToProto()
		err = cs.privValidator.SignVote(cs.state.ChainID, cs.Validators.QuorumType, cs.Validators.QuorumHash, p)
		if err != nil {
			t.Error(err)
		}
		precommit.BlockSignature = p.BlockSignature
		precommit.StateSignature = p.StateSignature
		cs.privValidator = nil // disable priv val so we don't do normal votes
		cs.mtx.Unlock()

		peers := sw.Peers().List()
		for _, peer := range peers {
			cs.Logger.Info("Sending bad vote", "block", blockHash, "peer", peer)
			peer.Send(VoteChannel, MustEncode(&VoteMessage{precommit}))
		}
	}()
}
