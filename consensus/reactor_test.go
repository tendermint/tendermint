package consensus

import (
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/tendermint/tendermint/config/tendermint_test"

	"github.com/tendermint/ed25519"
	. "github.com/tendermint/go-common"
	cfg "github.com/tendermint/go-config"
	"github.com/tendermint/go-crypto"
	dbm "github.com/tendermint/go-db"
	"github.com/tendermint/go-events"
	"github.com/tendermint/go-p2p"
	bc "github.com/tendermint/tendermint/blockchain"
	"github.com/tendermint/tendermint/types"
)

func init() {
	config = tendermint_test.ResetConfig("consensus_reactor_test")
}

func resetConfigTimeouts() {
	config.Set("log_level", "notice")
	config.Set("timeout_propose", 2000)
	//	config.Set("timeout_propose_delta", 500)
	//	config.Set("timeout_prevote", 1000)
	//	config.Set("timeout_prevote_delta", 500)
	//	config.Set("timeout_precommit", 1000)
	//	config.Set("timeout_precommit_delta", 500)
	//	config.Set("timeout_commit", 1000)
}

func _TestReactor(t *testing.T) {
	resetConfigTimeouts()
	N := 4
	css := randConsensusNet(N)
	reactors := make([]*ConsensusReactor, N)
	eventChans := make([]chan interface{}, N)
	for i := 0; i < N; i++ {
		blockStoreDB := dbm.NewDB(Fmt("blockstore%d", i), config.GetString("db_backend"), config.GetString("db_dir"))
		blockStore := bc.NewBlockStore(blockStoreDB)
		reactors[i] = NewConsensusReactor(css[i], blockStore, false)
		reactors[i].SetPrivValidator(css[i].privValidator)

		eventSwitch := events.NewEventSwitch()
		_, err := eventSwitch.Start()
		if err != nil {
			t.Fatalf("Failed to start switch: %v", err)
		}

		reactors[i].SetEventSwitch(eventSwitch)
		eventChans[i] = subscribeToEvent(eventSwitch, "tester", types.EventStringNewBlock(), 1)
	}
	p2p.MakeConnectedSwitches(N, func(i int, s *p2p.Switch) *p2p.Switch {
		s.AddReactor("CONSENSUS", reactors[i])
		return s
	}, net.Pipe)

	// wait till everyone makes the first new block
	wg := new(sync.WaitGroup)
	wg.Add(N)
	for i := 0; i < N; i++ {
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

	tick := time.NewTicker(time.Second * 3)
	select {
	case <-done:
	case <-tick.C:
		t.Fatalf("Timed out waiting for all validators to commit first block")
	}
}

func _TestByzantine(t *testing.T) {
	resetConfigTimeouts()
	N := 4
	css := randConsensusNet(N)

	switches := make([]*p2p.Switch, N)
	for i := 0; i < N; i++ {
		switches[i] = p2p.NewSwitch(cfg.NewMapConfig(nil))
	}

	reactors := make([]*ConsensusReactor, N)
	eventChans := make([]chan interface{}, N)
	for i := 0; i < N; i++ {
		blockStoreDB := dbm.NewDB(Fmt("blockstore%d", i), config.GetString("db_backend"), config.GetString("db_dir"))
		blockStore := bc.NewBlockStore(blockStoreDB)

		if i == 0 {
			// make byzantine
			css[i].decideProposal = func(j int) func(int, int) {
				return func(height, round int) {
					fmt.Println("hmph", j)
					byzantineDecideProposalFunc(height, round, css[j], switches[j])
				}
			}(i)
			css[i].doPrevote = func(height, round int) {}
			css[i].setProposal = func(j int) func(proposal *types.Proposal) error {
				return func(proposal *types.Proposal) error {
					return byzantineSetProposal(proposal, css[j], switches[j])
				}
			}(i)
		}
		reactors[i] = NewConsensusReactor(css[i], blockStore, false)
		reactors[i].SetPrivValidator(css[i].privValidator)

		eventSwitch := events.NewEventSwitch()
		_, err := eventSwitch.Start()
		if err != nil {
			t.Fatalf("Failed to start switch: %v", err)
		}

		reactors[i].SetEventSwitch(eventSwitch)
		eventChans[i] = subscribeToEvent(eventSwitch, "tester", types.EventStringNewBlock(), 1)
	}
	p2p.MakeConnectedSwitches(N, func(i int, s *p2p.Switch) *p2p.Switch {
		s.AddReactor("CONSENSUS", reactors[i])
		return s
	}, net.Pipe)

	// wait till everyone makes the first new block
	wg := new(sync.WaitGroup)
	wg.Add(N)
	for i := 0; i < N; i++ {
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

	tick := time.NewTicker(time.Second * 3)
	select {
	case <-done:
	case <-tick.C:
		t.Fatalf("Timed out waiting for all validators to commit first block")
	}
}

//-------------------------------
// byzantine consensus functions

func byzantineDecideProposalFunc(height, round int, cs *ConsensusState, sw *p2p.Switch) {
	// byzantine user should create two proposals and try to split the vote.
	// Avoid sending on internalMsgQueue and running consensus state.

	// Create a new proposal block from state/txs from the mempool.
	block1, blockParts1 := cs.createProposalBlock()
	polRound, polBlockID := cs.Votes.POLInfo()
	proposal1 := types.NewProposal(height, round, blockParts1.Header(), polRound, polBlockID)
	cs.privValidator.SignProposal(cs.state.ChainID, proposal1) // byzantine doesnt err

	// Create a new proposal block from state/txs from the mempool.
	block2, blockParts2 := cs.createProposalBlock()
	polRound, polBlockID = cs.Votes.POLInfo()
	proposal2 := types.NewProposal(height, round, blockParts2.Header(), polRound, polBlockID)
	cs.privValidator.SignProposal(cs.state.ChainID, proposal2) // byzantine doesnt err

	log.Notice("Byzantine: broadcasting conflicting proposals")
	// broadcast conflicting proposals/block parts to peers
	peers := sw.Peers().List()
	for i, peer := range peers {
		if i < len(peers)/2 {
			go sendProposalAndParts(height, round, cs, peer, proposal1, block1, blockParts1)
		} else {
			go sendProposalAndParts(height, round, cs, peer, proposal2, block2, blockParts2)
		}
	}
}

func sendProposalAndParts(height, round int, cs *ConsensusState, peer *p2p.Peer, proposal *types.Proposal, block *types.Block, parts *types.PartSet) {
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
	prevote, _ := cs.signVote(types.VoteTypePrevote, block.Hash(), parts.Header())
	peer.Send(VoteChannel, struct{ ConsensusMessage }{&VoteMessage{prevote}})
	precommit, _ := cs.signVote(types.VoteTypePrecommit, block.Hash(), parts.Header())
	peer.Send(VoteChannel, struct{ ConsensusMessage }{&VoteMessage{precommit}})
}

func byzantineSetProposal(proposal *types.Proposal, cs *ConsensusState, sw *p2p.Switch) error {
	peers := sw.Peers().List()
	for _, peer := range peers {
		// votes
		var blockHash []byte // XXX proposal.BlockHash
		blockHash = []byte{0, 1, 0, 2, 0, 3}
		prevote, _ := cs.signVote(types.VoteTypePrevote, blockHash, proposal.BlockPartsHeader)
		peer.Send(VoteChannel, struct{ ConsensusMessage }{&VoteMessage{prevote}})
		precommit, _ := cs.signVote(types.VoteTypePrecommit, blockHash, proposal.BlockPartsHeader)
		peer.Send(VoteChannel, struct{ ConsensusMessage }{&VoteMessage{precommit}})
	}
	return nil
}

//----------------------------------------
// byzantine privValidator
type ByzantinePrivValidator struct {
	Address []byte        `json:"address"`
	PubKey  crypto.PubKey `json:"pub_key"`

	// PrivKey should be empty if a Signer other than the default is being used.
	PrivKey      crypto.PrivKey `json:"priv_key"`
	types.Signer `json:"-"`

	mtx sync.Mutex
}

func (privVal *ByzantinePrivValidator) SetSigner(s types.Signer) {
	privVal.Signer = s
}

// Generates a new validator with private key.
func GenPrivValidator() *ByzantinePrivValidator {
	privKeyBytes := new([64]byte)
	copy(privKeyBytes[:32], crypto.CRandBytes(32))
	pubKeyBytes := ed25519.MakePublicKey(privKeyBytes)
	pubKey := crypto.PubKeyEd25519(*pubKeyBytes)
	privKey := crypto.PrivKeyEd25519(*privKeyBytes)
	return &ByzantinePrivValidator{
		Address: pubKey.Address(),
		PubKey:  pubKey,
		PrivKey: privKey,
		Signer:  types.NewDefaultSigner(privKey),
	}
}

func (privVal *ByzantinePrivValidator) GetAddress() []byte {
	return privVal.Address
}

func (privVal *ByzantinePrivValidator) SignVote(chainID string, vote *types.Vote) error {
	privVal.mtx.Lock()
	defer privVal.mtx.Unlock()

	// Sign
	vote.Signature = privVal.Sign(types.SignBytes(chainID, vote)).(crypto.SignatureEd25519)
	return nil
}

func (privVal *ByzantinePrivValidator) SignProposal(chainID string, proposal *types.Proposal) error {
	privVal.mtx.Lock()
	defer privVal.mtx.Unlock()

	// Sign
	proposal.Signature = privVal.Sign(types.SignBytes(chainID, proposal)).(crypto.SignatureEd25519)
	return nil
}

func (privVal *ByzantinePrivValidator) String() string {
	return Fmt("PrivValidator{%X}", privVal.Address)
}
