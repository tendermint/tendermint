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
	wire "github.com/tendermint/go-wire"
	auto "github.com/tendermint/tmlibs/autofile"
	cmn "github.com/tendermint/tmlibs/common"
	"github.com/tendermint/tmlibs/log"

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
		cs.Logger.Info("Replay: New Step", "height", m.Height, "round", m.Round, "step", m.Step)
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
			cs.Logger.Info("Replay: Proposal", "height", p.Height, "round", p.Round, "header",
				p.BlockPartsHeader, "pol", p.POLRound, "peer", peerKey)
		case *BlockPartMessage:
			cs.Logger.Info("Replay: BlockPart", "height", msg.Height, "round", msg.Round, "peer", peerKey)
		case *VoteMessage:
			v := msg.Vote
			cs.Logger.Info("Replay: Vote", "height", v.Height, "round", v.Round, "type", v.Type,
				"blockID", v.BlockID, "peer", peerKey)
		}

		cs.handleMsg(m, cs.RoundState)
	case timeoutInfo:
		cs.Logger.Info("Replay: Timeout", "height", m.Height, "round", m.Round, "step", m.Step, "dur", m.Duration)
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

	// Ensure that ENDHEIGHT for this height doesn't exist
	// NOTE: This is just a sanity check. As far as we know things work fine without it,
	// and Handshake could reuse ConsensusState if it weren't for this check (since we can crash after writing ENDHEIGHT).
	gr, found, err := cs.wal.group.Search("#ENDHEIGHT: ", makeHeightSearchFunc(csHeight))
	if gr != nil {
		gr.Close()
	}
	if found {
		return errors.New(cmn.Fmt("WAL should not contain #ENDHEIGHT %d.", csHeight))
	}

	// Search for last height marker
	gr, found, err = cs.wal.group.Search("#ENDHEIGHT: ", makeHeightSearchFunc(csHeight-1))
	if err == io.EOF {
		cs.Logger.Error("Replay: wal.group.Search returned EOF", "#ENDHEIGHT", csHeight-1)
		// if we upgraded from 0.9 to 0.9.1, we may have #HEIGHT instead
		// TODO (0.10.0): remove this
		gr, found, err = cs.wal.group.Search("#HEIGHT: ", makeHeightSearchFunc(csHeight))
		if err == io.EOF {
			cs.Logger.Error("Replay: wal.group.Search returned EOF", "#HEIGHT", csHeight)
			return nil
		} else if err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		defer gr.Close()
	}
	if !found {
		// if we upgraded from 0.9 to 0.9.1, we may have #HEIGHT instead
		// TODO (0.10.0): remove this
		gr, found, err = cs.wal.group.Search("#HEIGHT: ", makeHeightSearchFunc(csHeight))
		if err == io.EOF {
			cs.Logger.Error("Replay: wal.group.Search returned EOF", "#HEIGHT", csHeight)
			return nil
		} else if err != nil {
			return err
		} else {
			defer gr.Close()
		}

		// TODO (0.10.0): uncomment
		// return errors.New(cmn.Fmt("Cannot replay height %d. WAL does not contain #ENDHEIGHT for %d.", csHeight, csHeight-1))
	}

	cs.Logger.Info("Catchup by replaying consensus messages", "height", csHeight)

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
	cs.Logger.Info("Replay: Done")
	return nil
}

//--------------------------------------------------------------------------------

// Parses marker lines of the form:
// #ENDHEIGHT: 12345
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
	state  *sm.State
	store  types.BlockStore
	logger log.Logger

	nBlocks int // number of blocks applied to the state
}

func NewHandshaker(state *sm.State, store types.BlockStore) *Handshaker {
	return &Handshaker{state, store, log.NewNopLogger(), 0}
}

func (h *Handshaker) SetLogger(l log.Logger) {
	h.logger = l
}

func (h *Handshaker) NBlocks() int {
	return h.nBlocks
}

// TODO: retry the handshake/replay if it fails ?
func (h *Handshaker) Handshake(proxyApp proxy.AppConns) error {
	// handshake is done via info request on the query conn
	res, err := proxyApp.Query().InfoSync()
	if err != nil {
		return errors.New(cmn.Fmt("Error calling Info: %v", err))
	}

	blockHeight := int(res.LastBlockHeight) // XXX: beware overflow
	appHash := res.LastBlockAppHash

	h.logger.Info("ABCI Handshake", "appHeight", blockHeight, "appHash", fmt.Sprintf("%X", appHash))

	// TODO: check version

	// replay blocks up to the latest in the blockstore
	_, err = h.ReplayBlocks(appHash, blockHeight, proxyApp)
	if err != nil {
		return errors.New(cmn.Fmt("Error on replay: %v", err))
	}

	h.logger.Info("Completed ABCI Handshake - Tendermint and App are synced", "appHeight", blockHeight, "appHash", fmt.Sprintf("%X", appHash))

	// TODO: (on restart) replay mempool

	return nil
}

// Replay all blocks since appBlockHeight and ensure the result matches the current state.
// Returns the final AppHash or an error
func (h *Handshaker) ReplayBlocks(appHash []byte, appBlockHeight int, proxyApp proxy.AppConns) ([]byte, error) {

	storeBlockHeight := h.store.Height()
	stateBlockHeight := h.state.LastBlockHeight
	h.logger.Info("ABCI Replay Blocks", "appHeight", appBlockHeight, "storeHeight", storeBlockHeight, "stateHeight", stateBlockHeight)

	// If appBlockHeight == 0 it means that we are at genesis and hence should send InitChain
	if appBlockHeight == 0 {
		validators := types.TM2PB.Validators(h.state.Validators)
		proxyApp.Consensus().InitChainSync(validators)
	}

	// First handle edge cases and constraints on the storeBlockHeight
	if storeBlockHeight == 0 {
		return appHash, h.checkAppHash(appHash)

	} else if storeBlockHeight < appBlockHeight {
		// the app should never be ahead of the store (but this is under app's control)
		return appHash, sm.ErrAppBlockHeightTooHigh{storeBlockHeight, appBlockHeight}

	} else if storeBlockHeight < stateBlockHeight {
		// the state should never be ahead of the store (this is under tendermint's control)
		cmn.PanicSanity(cmn.Fmt("StateBlockHeight (%d) > StoreBlockHeight (%d)", stateBlockHeight, storeBlockHeight))

	} else if storeBlockHeight > stateBlockHeight+1 {
		// store should be at most one ahead of the state (this is under tendermint's control)
		cmn.PanicSanity(cmn.Fmt("StoreBlockHeight (%d) > StateBlockHeight + 1 (%d)", storeBlockHeight, stateBlockHeight+1))
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
			// We're good!
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
			// so replayBlock with the real app.
			// NOTE: We could instead use the cs.WAL on cs.Start,
			// but we'd have to allow the WAL to replay a block that wrote it's ENDHEIGHT
			h.logger.Info("Replay last block using real app")
			return h.replayBlock(storeBlockHeight, proxyApp.Consensus())

		} else if appBlockHeight == storeBlockHeight {
			// We ran Commit, but didn't save the state, so replayBlock with mock app
			abciResponses := h.state.LoadABCIResponses()
			mockApp := newMockProxyApp(appHash, abciResponses)
			h.logger.Info("Replay last block using mock app")
			return h.replayBlock(storeBlockHeight, mockApp)
		}

	}

	cmn.PanicSanity("Should never happen")
	return nil, nil
}

func (h *Handshaker) replayBlocks(proxyApp proxy.AppConns, appBlockHeight, storeBlockHeight int, mutateState bool) ([]byte, error) {
	// App is further behind than it should be, so we need to replay blocks.
	// We replay all blocks from appBlockHeight+1.
	// Note that we don't have an old version of the state,
	// so we by-pass state validation/mutation using sm.ExecCommitBlock.
	// If mutateState == true, the final block is replayed with h.replayBlock()

	var appHash []byte
	var err error
	finalBlock := storeBlockHeight
	if mutateState {
		finalBlock -= 1
	}
	for i := appBlockHeight + 1; i <= finalBlock; i++ {
		h.logger.Info("Applying block", "height", i)
		block := h.store.LoadBlock(i)
		appHash, err = sm.ExecCommitBlock(proxyApp.Consensus(), block, h.logger)
		if err != nil {
			return nil, err
		}

		h.nBlocks += 1
	}

	if mutateState {
		// sync the final block
		return h.replayBlock(storeBlockHeight, proxyApp.Consensus())
	}

	return appHash, h.checkAppHash(appHash)
}

// ApplyBlock on the proxyApp with the last block.
func (h *Handshaker) replayBlock(height int, proxyApp proxy.AppConnConsensus) ([]byte, error) {
	mempool := types.MockMempool{}

	var eventCache types.Fireable // nil
	block := h.store.LoadBlock(height)
	meta := h.store.LoadBlockMeta(height)

	if err := h.state.ApplyBlock(eventCache, proxyApp, block, meta.BlockID.PartsHeader, mempool); err != nil {
		return nil, err
	}

	h.nBlocks += 1

	return h.state.AppHash, nil
}

func (h *Handshaker) checkAppHash(appHash []byte) error {
	if !bytes.Equal(h.state.AppHash, appHash) {
		panic(errors.New(cmn.Fmt("Tendermint state.AppHash does not match AppHash after replay. Got %X, expected %X", appHash, h.state.AppHash)).Error())
		return nil
	}
	return nil
}

//--------------------------------------------------------------------------------
// mockProxyApp uses ABCIResponses to give the right results
// Useful because we don't want to call Commit() twice for the same block on the real app.

func newMockProxyApp(appHash []byte, abciResponses *sm.ABCIResponses) proxy.AppConnConsensus {
	clientCreator := proxy.NewLocalClientCreator(&mockProxyApp{
		appHash:       appHash,
		abciResponses: abciResponses,
	})
	cli, _ := clientCreator.NewABCIClient()
	cli.Start()
	return proxy.NewAppConnConsensus(cli)
}

type mockProxyApp struct {
	abci.BaseApplication

	appHash       []byte
	txCount       int
	abciResponses *sm.ABCIResponses
}

func (mock *mockProxyApp) DeliverTx(tx []byte) abci.Result {
	r := mock.abciResponses.DeliverTx[mock.txCount]
	mock.txCount += 1
	return abci.Result{
		r.Code,
		r.Data,
		r.Log,
	}
}

func (mock *mockProxyApp) EndBlock(height uint64) abci.ResponseEndBlock {
	mock.txCount = 0
	return mock.abciResponses.EndBlock
}

func (mock *mockProxyApp) Commit() abci.Result {
	return abci.NewResultOK(mock.appHash, "")
}
