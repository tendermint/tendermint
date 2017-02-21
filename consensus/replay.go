package consensus

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"time"

	abci "github.com/tendermint/abci/types"
	auto "github.com/tendermint/go-autofile"
	. "github.com/tendermint/go-common"
	cfg "github.com/tendermint/go-config"
	"github.com/tendermint/go-wire"

	"github.com/tendermint/tendermint/proxy"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

// Functionality to replay blocks and messages on recovery from a crash.
// There are two general failure scenarios: failure during consensus, and failure while applying the block.
// The former is handled by the WAL, the latter by the proxyApp Handshake on restart,
// which ultimately hands off the work to the WAL.

//-----------------------------------------
// recover from failure during consensus
// by replaying messages from the WAL

// Unmarshal and apply a single message to the consensus state
// as if it were received in receiveRoutine
// Lines that start with "#" are ignored.
// NOTE: receiveRoutine should not be running
func (cs *ConsensusState) readReplayMessage(msgBytes []byte, newStepCh chan interface{}) error {
	// Skip over empty and meta lines
	if len(msgBytes) == 0 || msgBytes[0] == '#' {
		return nil
	}
	var err error
	var msg TimedWALMessage
	wire.ReadJSON(&msg, msgBytes, &err)
	if err != nil {
		fmt.Println("MsgBytes:", msgBytes, string(msgBytes))
		return fmt.Errorf("Error reading json data: %v", err)
	}

	// for logging
	switch m := msg.Msg.(type) {
	case types.EventDataRoundState:
		log.Notice("Replay: New Step", "height", m.Height, "round", m.Round, "step", m.Step)
		// these are playback checks
		ticker := time.After(time.Second * 2)
		if newStepCh != nil {
			select {
			case mi := <-newStepCh:
				m2 := mi.(types.EventDataRoundState)
				if m.Height != m2.Height || m.Round != m2.Round || m.Step != m2.Step {
					return fmt.Errorf("RoundState mismatch. Got %v; Expected %v", m2, m)
				}
			case <-ticker:
				return fmt.Errorf("Failed to read off newStepCh")
			}
		}
	case msgInfo:
		peerKey := m.PeerKey
		if peerKey == "" {
			peerKey = "local"
		}
		switch msg := m.Msg.(type) {
		case *ProposalMessage:
			p := msg.Proposal
			log.Notice("Replay: Proposal", "height", p.Height, "round", p.Round, "header",
				p.BlockPartsHeader, "pol", p.POLRound, "peer", peerKey)
		case *BlockPartMessage:
			log.Notice("Replay: BlockPart", "height", msg.Height, "round", msg.Round, "peer", peerKey)
		case *VoteMessage:
			v := msg.Vote
			log.Notice("Replay: Vote", "height", v.Height, "round", v.Round, "type", v.Type,
				"blockID", v.BlockID, "peer", peerKey)
		}

		cs.handleMsg(m, cs.RoundState)
	case timeoutInfo:
		log.Notice("Replay: Timeout", "height", m.Height, "round", m.Round, "step", m.Step, "dur", m.Duration)
		cs.handleTimeout(m, cs.RoundState)
	default:
		return fmt.Errorf("Replay: Unknown TimedWALMessage type: %v", reflect.TypeOf(msg.Msg))
	}
	return nil
}

// replay only those messages since the last block.
// timeoutRoutine should run concurrently to read off tickChan
func (cs *ConsensusState) catchupReplay(csHeight int) error {

	// set replayMode
	cs.replayMode = true
	defer func() { cs.replayMode = false }()

	// Ensure that height+1 doesn't exist
	gr, found, err := cs.wal.group.Search("#HEIGHT: ", makeHeightSearchFunc(csHeight+1))
	if found {
		return errors.New(Fmt("WAL should not contain height %d.", csHeight+1))
	}
	if gr != nil {
		gr.Close()
	}

	// Search for height marker
	gr, found, err = cs.wal.group.Search("#HEIGHT: ", makeHeightSearchFunc(csHeight))
	if err == io.EOF {
		log.Warn("Replay: wal.group.Search returned EOF", "height", csHeight)
		return nil
	} else if err != nil {
		return err
	}
	if !found {
		return errors.New(Fmt("WAL does not contain height %d.", csHeight))
	}
	defer gr.Close()

	log.Notice("Catchup by replaying consensus messages", "height", csHeight)

	for {
		line, err := gr.ReadLine()
		if err != nil {
			if err == io.EOF {
				break
			} else {
				return err
			}
		}
		// NOTE: since the priv key is set when the msgs are received
		// it will attempt to eg double sign but we can just ignore it
		// since the votes will be replayed and we'll get to the next step
		if err := cs.readReplayMessage([]byte(line), nil); err != nil {
			return err
		}
	}
	log.Notice("Replay: Done")
	return nil
}

//--------------------------------------------------------------------------------

// Parses marker lines of the form:
// #HEIGHT: 12345
func makeHeightSearchFunc(height int) auto.SearchFunc {
	return func(line string) (int, error) {
		line = strings.TrimRight(line, "\n")
		parts := strings.Split(line, " ")
		if len(parts) != 2 {
			return -1, errors.New("Line did not have 2 parts")
		}
		i, err := strconv.Atoi(parts[1])
		if err != nil {
			return -1, errors.New("Failed to parse INFO: " + err.Error())
		}
		if height < i {
			return 1, nil
		} else if height == i {
			return 0, nil
		} else {
			return -1, nil
		}
	}
}

//----------------------------------------------
// Recover from failure during block processing
// by handshaking with the app to figure out where
// we were last and using the WAL to recover there

type Handshaker struct {
	config cfg.Config
	state  *sm.State
	store  types.BlockStore

	nBlocks int // number of blocks applied to the state
}

func NewHandshaker(config cfg.Config, state *sm.State, store types.BlockStore) *Handshaker {
	return &Handshaker{config, state, store, 0}
}

func (h *Handshaker) NBlocks() int {
	return h.nBlocks
}

// TODO: retry the handshake/replay if it fails ?
func (h *Handshaker) Handshake(proxyApp proxy.AppConns) error {
	// handshake is done via info request on the query conn
	res, err := proxyApp.Query().InfoSync()
	if err != nil {
		return errors.New(Fmt("Error calling Info: %v", err))
	}

	blockHeight := int(res.LastBlockHeight) // XXX: beware overflow
	appHash := res.LastBlockAppHash

	log.Notice("ABCI Handshake", "appHeight", blockHeight, "appHash", appHash)

	// TODO: check version

	// replay blocks up to the latest in the blockstore
	_, err = h.ReplayBlocks(appHash, blockHeight, proxyApp)
	if err != nil {
		return errors.New(Fmt("Error on replay: %v", err))
	}

	// TODO: (on restart) replay mempool

	return nil
}

// Replay all blocks since appBlockHeight and ensure the result matches the current state.
// Returns the final AppHash or an error
func (h *Handshaker) ReplayBlocks(appHash []byte, appBlockHeight int, proxyApp proxy.AppConns) ([]byte, error) {

	storeBlockHeight := h.store.Height()
	stateBlockHeight := h.state.LastBlockHeight
	log.Notice("ABCI Replay Blocks", "appHeight", appBlockHeight, "storeHeight", storeBlockHeight, "stateHeight", stateBlockHeight)

	// First handle edge cases and constraints on the storeBlockHeight
	if storeBlockHeight == 0 {
		return appHash, h.checkAppHash(appHash)

	} else if storeBlockHeight < appBlockHeight {
		// the app should never be ahead of the store (but this is under app's control)
		return appHash, sm.ErrAppBlockHeightTooHigh{storeBlockHeight, appBlockHeight}

	} else if storeBlockHeight < stateBlockHeight {
		// the state should never be ahead of the store (this is under tendermint's control)
		PanicSanity(Fmt("StateBlockHeight (%d) > StoreBlockHeight (%d)", stateBlockHeight, storeBlockHeight))

	} else if storeBlockHeight > stateBlockHeight+1 {
		// store should be at most one ahead of the state (this is under tendermint's control)
		PanicSanity(Fmt("StoreBlockHeight (%d) > StateBlockHeight + 1 (%d)", storeBlockHeight, stateBlockHeight+1))
	}

	// Now either store is equal to state, or one ahead.
	// For each, consider all cases of where the app could be, given app <= store
	if storeBlockHeight == stateBlockHeight {
		// Tendermint ran Commit and saved the state.
		// Either the app is asking for replay, or we're all synced up.
		if appBlockHeight < storeBlockHeight {
			// the app is behind, so replay blocks, but no need to go through WAL (state is already synced to store)
			return h.replayBlocks(proxyApp, appBlockHeight, storeBlockHeight, false)

		} else if appBlockHeight == storeBlockHeight {
			// we're good!
			return appHash, h.checkAppHash(appHash)
		}

	} else if storeBlockHeight == stateBlockHeight+1 {
		// We saved the block in the store but haven't updated the state,
		// so we'll need to replay a block using the WAL.
		if appBlockHeight < stateBlockHeight {
			// the app is further behind than it should be, so replay blocks
			// but leave the last block to go through the WAL
			return h.replayBlocks(proxyApp, appBlockHeight, storeBlockHeight, true)

		} else if appBlockHeight == stateBlockHeight {
			// We haven't run Commit (both the state and app are one block behind),
			// so run through consensus with the real app
			log.Info("Replay last block using real app")
			return h.replayLastBlock(proxyApp.Consensus())

		} else if appBlockHeight == storeBlockHeight {
			// We ran Commit, but didn't save the state, so run through consensus with mock app
			mockApp := newMockProxyApp(appHash)
			log.Info("Replay last block using mock app")
			return h.replayLastBlock(mockApp)
		}

	}

	PanicSanity("Should never happen")
	return nil, nil
}

func (h *Handshaker) replayBlocks(proxyApp proxy.AppConns, appBlockHeight, storeBlockHeight int, useReplayFunc bool) ([]byte, error) {
	// App is further behind than it should be, so we need to replay blocks.
	// We replay all blocks from appBlockHeight+1 to storeBlockHeight-1,
	// and let the final block be replayed through ReplayBlocks.
	// Note that we don't have an old version of the state,
	// so we by-pass state validation using applyBlock here.

	var appHash []byte
	var err error
	finalBlock := storeBlockHeight
	if useReplayFunc {
		finalBlock -= 1
	}
	for i := appBlockHeight + 1; i <= finalBlock; i++ {
		log.Info("Applying block", "height", i)
		block := h.store.LoadBlock(i)
		appHash, err = sm.ApplyBlock(proxyApp.Consensus(), block)
		if err != nil {
			return nil, err
		}

		h.nBlocks += 1
	}

	if useReplayFunc {
		// sync the final block
		return h.ReplayBlocks(appHash, finalBlock, proxyApp)
	}

	return appHash, h.checkAppHash(appHash)
}

// Replay the last block through the consensus and return the AppHash from after Commit.
func (h *Handshaker) replayLastBlock(proxyApp proxy.AppConnConsensus) ([]byte, error) {
	mempool := types.MockMempool{}
	cs := NewConsensusState(h.config, h.state, proxyApp, h.store, mempool)

	evsw := types.NewEventSwitch()
	evsw.Start()
	defer evsw.Stop()
	cs.SetEventSwitch(evsw)
	newBlockCh := subscribeToEvent(evsw, "consensus-replay", types.EventStringNewBlock(), 1)

	// run through the WAL, commit new block, stop
	cs.Start()
	<-newBlockCh // TODO: use a timeout and return err?
	cs.Stop()

	h.nBlocks += 1

	return cs.state.AppHash, nil
}

func (h *Handshaker) checkAppHash(appHash []byte) error {
	if !bytes.Equal(h.state.AppHash, appHash) {
		panic(errors.New(Fmt("Tendermint state.AppHash does not match AppHash after replay. Got %X, expected %X", appHash, h.state.AppHash)).Error())
		return nil
	}
	return nil
}

//--------------------------------------------------------------------------------

func newMockProxyApp(appHash []byte) proxy.AppConnConsensus {
	clientCreator := proxy.NewLocalClientCreator(&mockProxyApp{appHash: appHash})
	cli, _ := clientCreator.NewABCIClient()
	return proxy.NewAppConnConsensus(cli)
}

type mockProxyApp struct {
	abci.BaseApplication

	appHash []byte
}

func (mock *mockProxyApp) Commit() abci.Result {
	return abci.NewResultOK(mock.appHash, "")
}
