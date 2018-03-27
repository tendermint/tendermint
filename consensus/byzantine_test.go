package consensus

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	crypto "github.com/tendermint/go-crypto"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/types"
	cmn "github.com/tendermint/tmlibs/common"
)

func init() {
	config = ResetConfig("consensus_byzantine_test")
}

//----------------------------------------------
// byzantine failures

// 4 validators. 1 is byzantine. The other three are partitioned into A (1 val) and B (2 vals).
// byzantine validator sends conflicting proposals into A and B,
// and prevotes/precommits on both of them.
// B sees a commit, A doesn't.
// Byzantine validator refuses to prevote.
// Heal partition and ensure A sees the commit
func TestByzantine(t *testing.T) {
	N := 4
	logger := consensusLogger()
	css := randConsensusNet(N, "consensus_byzantine_test", newMockTickerFunc(false), newCounter)

	// give the byzantine validator a normal ticker
	ticker := NewTimeoutTicker()
	ticker.SetLogger(css[0].Logger)
	css[0].SetTimeoutTicker(ticker)

	switches := make([]*p2p.Switch, N)
	p2pLogger := logger.With("module", "p2p")
	for i := 0; i < N; i++ {
		switches[i] = p2p.NewSwitch(config.P2P)
		switches[i].SetLogger(p2pLogger.With("validator", i))
	}

	eventChans := make([]chan interface{}, N)
	reactors := make([]p2p.Reactor, N)
	for i := 0; i < N; i++ {
		// make first val byzantine
		if i == 0 {
			css[i].privValidator = NewByzantinePrivValidator(css[i].privValidator)
			css[i].decideProposal = func(j int) func(int64, int) {
				return func(height int64, round int) {
					byzantineDecideProposalFunc(t, height, round, css[j], switches[j])
				}
			}(i)
			css[i].doPrevote = func(height int64, round int) {}
		}

		eventBus := types.NewEventBus()
		eventBus.SetLogger(logger.With("module", "events", "validator", i))
		err := eventBus.Start()
		require.NoError(t, err)
		defer eventBus.Stop()

		eventChans[i] = make(chan interface{}, 1)
		err = eventBus.Subscribe(context.Background(), testSubscriber, types.EventQueryNewBlock, eventChans[i])
		require.NoError(t, err)

		conR := NewConsensusReactor(css[i], true) // so we dont start the consensus states
		conR.SetLogger(logger.With("validator", i))
		conR.SetEventBus(eventBus)

		var conRI p2p.Reactor // nolint: gotype, gosimple
		conRI = conR

		// make first val byzantine
		if i == 0 {
			conRI = NewByzantineReactor(conR)
		}

		reactors[i] = conRI
	}

	defer func() {
		for _, r := range reactors {
			if rr, ok := r.(*ByzantineReactor); ok {
				rr.reactor.Switch.Stop()
			} else {
				r.(*ConsensusReactor).Switch.Stop()
			}
		}
	}()

	p2p.MakeConnectedSwitches(config.P2P, N, func(i int, s *p2p.Switch) *p2p.Switch {
		// ignore new switch s, we already made ours
		switches[i].AddReactor("CONSENSUS", reactors[i])
		return switches[i]
	}, func(sws []*p2p.Switch, i, j int) {
		// the network starts partitioned with globally active adversary
		if i != 0 {
			return
		}
		p2p.Connect2Switches(sws, i, j)
	})

	// start the state machines
	byzR := reactors[0].(*ByzantineReactor)
	s := byzR.reactor.conS.GetState()
	byzR.reactor.SwitchToConsensus(s, 0)
	for i := 1; i < N; i++ {
		cr := reactors[i].(*ConsensusReactor)
		cr.SwitchToConsensus(cr.conS.GetState(), 0)
	}

	// byz proposer sends one block to peers[0]
	// and the other block to peers[1] and peers[2].
	// note peers and switches order don't match.
	peers := switches[0].Peers().List()

	// partition A
	ind0 := getSwitchIndex(switches, peers[0])

	// partition B
	ind1 := getSwitchIndex(switches, peers[1])
	ind2 := getSwitchIndex(switches, peers[2])
	p2p.Connect2Switches(switches, ind1, ind2)

	// wait for someone in the big partition (B) to make a block
	<-eventChans[ind2]

	t.Log("A block has been committed. Healing partition")
	p2p.Connect2Switches(switches, ind0, ind1)
	p2p.Connect2Switches(switches, ind0, ind2)

	// wait till everyone makes the first new block
	// (one of them already has)
	wg := new(sync.WaitGroup)
	wg.Add(2)
	for i := 1; i < N-1; i++ {
		go func(j int) {
			<-eventChans[j]
			wg.Done()
		}(i)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	tick := time.NewTicker(time.Second * 10)
	select {
	case <-done:
	case <-tick.C:
		for i, reactor := range reactors {
			t.Log(cmn.Fmt("Consensus Reactor %v", i))
			t.Log(cmn.Fmt("%v", reactor))
		}
		t.Fatalf("Timed out waiting for all validators to commit first block")
	}
}

//-------------------------------
// byzantine consensus functions

func byzantineDecideProposalFunc(t *testing.T, height int64, round int, cs *ConsensusState, sw *p2p.Switch) {
	// byzantine user should create two proposals and try to split the vote.
	// Avoid sending on internalMsgQueue and running consensus state.

	// Create a new proposal block from state/txs from the mempool.
	block1, blockParts1 := cs.createProposalBlock()
	polRound, polBlockID := cs.Votes.POLInfo()
	proposal1 := types.NewProposal(height, round, blockParts1.Header(), polRound, polBlockID)
	if err := cs.privValidator.SignProposal(cs.state.ChainID, proposal1); err != nil {
		t.Error(err)
	}

	// Create a new proposal block from state/txs from the mempool.
	block2, blockParts2 := cs.createProposalBlock()
	polRound, polBlockID = cs.Votes.POLInfo()
	proposal2 := types.NewProposal(height, round, blockParts2.Header(), polRound, polBlockID)
	if err := cs.privValidator.SignProposal(cs.state.ChainID, proposal2); err != nil {
		t.Error(err)
	}

	block1Hash := block1.Hash()
	block2Hash := block2.Hash()

	// broadcast conflicting proposals/block parts to peers
	peers := sw.Peers().List()
	t.Logf("Byzantine: broadcasting conflicting proposals to %d peers", len(peers))
	for i, peer := range peers {
		if i < len(peers)/2 {
			go sendProposalAndParts(height, round, cs, peer, proposal1, block1Hash, blockParts1)
		} else {
			go sendProposalAndParts(height, round, cs, peer, proposal2, block2Hash, blockParts2)
		}
	}
}

func sendProposalAndParts(height int64, round int, cs *ConsensusState, peer p2p.Peer, proposal *types.Proposal, blockHash []byte, parts *types.PartSet) {
	// proposal
	msg := &ProposalMessage{Proposal: proposal}
	peer.Send(DataChannel, struct{ ConsensusMessage }{msg})

	// parts
	for i := 0; i < parts.Total(); i++ {
		part := parts.GetPart(i)
		msg := &BlockPartMessage{
			Height: height, // This tells peer that this part applies to us.
			Round:  round,  // This tells peer that this part applies to us.
			Part:   part,
		}
		peer.Send(DataChannel, struct{ ConsensusMessage }{msg})
	}

	// votes
	cs.mtx.Lock()
	prevote, _ := cs.signVote(types.VoteTypePrevote, blockHash, parts.Header())
	precommit, _ := cs.signVote(types.VoteTypePrecommit, blockHash, parts.Header())
	cs.mtx.Unlock()

	peer.Send(VoteChannel, struct{ ConsensusMessage }{&VoteMessage{prevote}})
	peer.Send(VoteChannel, struct{ ConsensusMessage }{&VoteMessage{precommit}})
}

//----------------------------------------
// byzantine consensus reactor

type ByzantineReactor struct {
	cmn.Service
	reactor *ConsensusReactor
}

func NewByzantineReactor(conR *ConsensusReactor) *ByzantineReactor {
	return &ByzantineReactor{
		Service: conR,
		reactor: conR,
	}
}

func (br *ByzantineReactor) SetSwitch(s *p2p.Switch)               { br.reactor.SetSwitch(s) }
func (br *ByzantineReactor) GetChannels() []*p2p.ChannelDescriptor { return br.reactor.GetChannels() }
func (br *ByzantineReactor) AddPeer(peer p2p.Peer) {
	if !br.reactor.IsRunning() {
		return
	}

	// Create peerState for peer
	peerState := NewPeerState(peer).SetLogger(br.reactor.Logger)
	peer.Set(types.PeerStateKey, peerState)

	// Send our state to peer.
	// If we're fast_syncing, broadcast a RoundStepMessage later upon SwitchToConsensus().
	if !br.reactor.fastSync {
		br.reactor.sendNewRoundStepMessages(peer)
	}
}
func (br *ByzantineReactor) RemovePeer(peer p2p.Peer, reason interface{}) {
	br.reactor.RemovePeer(peer, reason)
}
func (br *ByzantineReactor) Receive(chID byte, peer p2p.Peer, msgBytes []byte) {
	br.reactor.Receive(chID, peer, msgBytes)
}

//----------------------------------------
// byzantine privValidator

type ByzantinePrivValidator struct {
	types.Signer

	pv types.PrivValidator
}

// Return a priv validator that will sign anything
func NewByzantinePrivValidator(pv types.PrivValidator) *ByzantinePrivValidator {
	return &ByzantinePrivValidator{
		Signer: pv.(*types.PrivValidatorFS).Signer,
		pv:     pv,
	}
}

func (privVal *ByzantinePrivValidator) GetAddress() types.Address {
	return privVal.pv.GetAddress()
}

func (privVal *ByzantinePrivValidator) GetPubKey() crypto.PubKey {
	return privVal.pv.GetPubKey()
}

func (privVal *ByzantinePrivValidator) SignVote(chainID string, vote *types.Vote) (err error) {
	vote.Signature, err = privVal.Sign(vote.SignBytes(chainID))
	return err
}

func (privVal *ByzantinePrivValidator) SignProposal(chainID string, proposal *types.Proposal) (err error) {
	proposal.Signature, _ = privVal.Sign(proposal.SignBytes(chainID))
	return nil
}

func (privVal *ByzantinePrivValidator) SignHeartbeat(chainID string, heartbeat *types.Heartbeat) (err error) {
	heartbeat.Signature, _ = privVal.Sign(heartbeat.SignBytes(chainID))
	return nil
}

func (privVal *ByzantinePrivValidator) String() string {
	return cmn.Fmt("PrivValidator{%X}", privVal.GetAddress())
}
