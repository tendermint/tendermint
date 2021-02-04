package consensus

import (
	"bytes"
	"context"
	"fmt"
	"hash/crc32"
	"io"
	"reflect"
	"time"

	abci "github.com/tendermint/tendermint/abci/types"
	tmcon "github.com/tendermint/tendermint/consensus"
	"github.com/tendermint/tendermint/crypto/merkle"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/proxy"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

var crc32c = crc32.MakeTable(crc32.Castagnoli)

// Functionality to replay blocks and messages on recovery from a crash.
// There are two general failure scenarios:
//
//  1. failure during consensus
//  2. failure while applying the block
//
// The former is handled by the WAL, the latter by the proxyApp Handshake on
// restart, which ultimately hands off the work to the WAL.

//-----------------------------------------
// 1. Recover from failure during consensus
// (by replaying messages from the WAL)
//-----------------------------------------

// Unmarshal and apply a single message to the consensus state as if it were
// received in receiveRoutine.  Lines that start with "#" are ignored.
// NOTE: receiveRoutine should not be running.
func (cs *State) readReplayMessage(msg *tmcon.TimedWALMessage, newStepSub types.Subscription) error {
	// Skip meta messages which exist for demarcating boundaries.
	if _, ok := msg.Msg.(tmcon.EndHeightMessage); ok {
		return nil
	}

	// for logging
	switch m := msg.Msg.(type) {
	case types.EventDataRoundState:
		cs.Logger.Info("Replay: New Step", "height", m.Height, "round", m.Round, "step", m.Step)
		// these are playback checks
		ticker := time.After(time.Second * 2)
		if newStepSub != nil {
			select {
			case stepMsg := <-newStepSub.Out():
				m2 := stepMsg.Data().(types.EventDataRoundState)
				if m.Height != m2.Height || m.Round != m2.Round || m.Step != m2.Step {
					return fmt.Errorf("roundState mismatch. Got %v; Expected %v", m2, m)
				}
			case <-newStepSub.Cancelled():
				return fmt.Errorf("failed to read off newStepSub.Out(). newStepSub was cancelled")
			case <-ticker:
				return fmt.Errorf("failed to read off newStepSub.Out()")
			}
		}
	case msgInfo:
		peerID := m.PeerID
		if peerID == "" {
			peerID = "local"
		}
		switch msg := m.Msg.(type) {
		case *tmcon.ProposalMessage:
			p := msg.Proposal
			cs.Logger.Info("Replay: Proposal", "height", p.Height, "round", p.Round, "header",
				p.BlockID.PartSetHeader, "pol", p.POLRound, "peer", peerID)
		case *tmcon.BlockPartMessage:
			cs.Logger.Info("Replay: BlockPart", "height", msg.Height, "round", msg.Round, "peer", peerID)
		case *tmcon.VoteMessage:
			v := msg.Vote
			cs.Logger.Info("Replay: Vote", "height", v.Height, "round", v.Round, "type", v.Type,
				"blockID", v.BlockID, "peer", peerID)
		}

		cs.handleMsg(m)
	case timeoutInfo:
		cs.Logger.Info("Replay: Timeout", "height", m.Height, "round", m.Round, "step", m.Step, "dur", m.Duration)
		cs.handleTimeout(m, cs.RoundState)
	default:
		return fmt.Errorf("replay: Unknown TimedWALMessage type: %v", reflect.TypeOf(msg.Msg))
	}
	return nil
}

// Replay only those messages since the last block.  `timeoutRoutine` should
// run concurrently to read off tickChan.
func (cs *State) catchupReplay(csHeight int64) error {

	// Set replayMode to true so we don't log signing errors.
	cs.replayMode = true
	defer func() { cs.replayMode = false }()

	// Ensure that #ENDHEIGHT for this height doesn't exist.
	// NOTE: This is just a sanity check. As far as we know things work fine
	// without it, and Handshake could reuse State if it weren't for
	// this check (since we can crash after writing #ENDHEIGHT).
	//
	// Ignore data corruption errors since this is a sanity check.
	gr, found, err := cs.wal.SearchForEndHeight(csHeight, &tmcon.WALSearchOptions{IgnoreDataCorruptionErrors: true})
	if err != nil {
		return err
	}
	if gr != nil {
		if err := gr.Close(); err != nil {
			return err
		}
	}
	if found {
		return fmt.Errorf("wal should not contain #ENDHEIGHT %d", csHeight)
	}

	// Search for last height marker.
	//
	// Ignore data corruption errors in previous heights because we only care about last height
	if csHeight < cs.state.InitialHeight {
		return fmt.Errorf("cannot replay height %v, below initial height %v", csHeight, cs.state.InitialHeight)
	}
	endHeight := csHeight - 1
	if csHeight == cs.state.InitialHeight {
		endHeight = 0
	}
	gr, found, err = cs.wal.SearchForEndHeight(endHeight, &tmcon.WALSearchOptions{IgnoreDataCorruptionErrors: true})
	if err == io.EOF {
		cs.Logger.Error("Replay: wal.group.Search returned EOF", "#ENDHEIGHT", endHeight)
	} else if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("cannot replay height %d. WAL does not contain #ENDHEIGHT for %d", csHeight, endHeight)
	}
	defer gr.Close()

	cs.Logger.Info("Catchup by replaying consensus messages", "height", csHeight)

	var msg *tmcon.TimedWALMessage
	dec := WALDecoder{gr}

LOOP:
	for {
		msg, err = dec.Decode()
		switch {
		case err == io.EOF:
			break LOOP
		case IsDataCorruptionError(err):
			cs.Logger.Error("data has been corrupted in last height of consensus WAL", "err", err, "height", csHeight)
			return err
		case err != nil:
			return err
		}

		// NOTE: since the priv key is set when the msgs are received
		// it will attempt to eg double sign but we can just ignore it
		// since the votes will be replayed and we'll get to the next step
		if err := cs.readReplayMessage(msg, nil); err != nil {
			return err
		}
	}
	cs.Logger.Info("Replay: Done")
	return nil
}

//--------------------------------------------------------------------------------

// Parses marker lines of the form:
// #ENDHEIGHT: 12345
/*
func makeHeightSearchFunc(height int64) auto.SearchFunc {
	return func(line string) (int, error) {
		line = strings.TrimRight(line, "\n")
		parts := strings.Split(line, " ")
		if len(parts) != 2 {
			return -1, errors.New("line did not have 2 parts")
		}
		i, err := strconv.Atoi(parts[1])
		if err != nil {
			return -1, errors.New("failed to parse INFO: " + err.Error())
		}
		if height < i {
			return 1, nil
		} else if height == i {
			return 0, nil
		} else {
			return -1, nil
		}
	}
}*/

//---------------------------------------------------
// 2. Recover from failure while applying the block.
// (by handshaking with the app to figure out where
// we were last, and using the WAL to recover there.)
//---------------------------------------------------

type Handshaker struct {
	stateStore   sm.Store
	initialState sm.State
	store        sm.BlockStore
	eventBus     types.BlockEventPublisher
	genDoc       *types.GenesisDoc
	logger       log.Logger

	nBlocks int // number of blocks applied to the state
}

func NewHandshaker(stateStore sm.Store, state sm.State,
	store sm.BlockStore, genDoc *types.GenesisDoc) *Handshaker {

	return &Handshaker{
		stateStore:   stateStore,
		initialState: state,
		store:        store,
		eventBus:     types.NopEventBus{},
		genDoc:       genDoc,
		logger:       log.NewNopLogger(),
		nBlocks:      0,
	}
}

func (h *Handshaker) SetLogger(l log.Logger) {
	h.logger = l
}

// SetEventBus - sets the event bus for publishing block related events.
// If not called, it defaults to types.NopEventBus.
func (h *Handshaker) SetEventBus(eventBus types.BlockEventPublisher) {
	h.eventBus = eventBus
}

// NBlocks returns the number of blocks applied to the state.
func (h *Handshaker) NBlocks() int {
	return h.nBlocks
}

// TODO: retry the handshake/replay if it fails ?
func (h *Handshaker) Handshake(proxyApp proxy.AppConns) error {

	// Handshake is done via ABCI Info on the query conn.
	res, err := proxyApp.Query().InfoSync(context.Background(), proxy.RequestInfo)
	if err != nil {
		return fmt.Errorf("error calling Info: %v", err)
	}

	blockHeight := res.LastBlockHeight
	if blockHeight < 0 {
		return fmt.Errorf("got a negative last block height (%d) from the app", blockHeight)
	}
	appHash := res.LastBlockAppHash

	h.logger.Info("ABCI Handshake App Info",
		"height", blockHeight,
		"hash", appHash,
		"software-version", res.Version,
		"protocol-version", res.AppVersion,
	)

	// Only set the version if there is no existing state.
	if h.initialState.LastBlockHeight == 0 {
		h.initialState.Version.Consensus.App = res.AppVersion
	}

	// Replay blocks up to the latest in the blockstore.
	_, err = h.ReplayBlocks(h.initialState, appHash, blockHeight, proxyApp)
	if err != nil {
		return fmt.Errorf("error on replay: %v", err)
	}

	h.logger.Info("Completed ABCI Handshake - Tendermint and App are synced",
		"appHeight", blockHeight, "appHash", appHash)

	// TODO: (on restart) replay mempool

	return nil
}

// ReplayBlocks replays all blocks since appBlockHeight and ensures the result
// matches the current state.
// Returns the final AppHash or an error.
func (h *Handshaker) ReplayBlocks(
	state sm.State,
	appHash []byte,
	appBlockHeight int64,
	proxyApp proxy.AppConns,
) ([]byte, error) {
	storeBlockBase := h.store.Base()
	storeBlockHeight := h.store.Height()
	stateBlockHeight := state.LastBlockHeight
	h.logger.Info(
		"ABCI Replay Blocks",
		"appHeight",
		appBlockHeight,
		"storeHeight",
		storeBlockHeight,
		"stateHeight",
		stateBlockHeight)

	// If appBlockHeight == 0 it means that we are at genesis and hence should send InitChain.
	if appBlockHeight == 0 {
		validators := make([]*types.Validator, len(h.genDoc.Validators))
		for i, val := range h.genDoc.Validators {
			validators[i] = types.NewValidator(val.PubKey, val.Power)
		}
		validatorSet := types.NewValidatorSet(validators)
		nextVals := types.TM2PB.ValidatorUpdates(validatorSet)
		pbParams := h.genDoc.ConsensusParams.ToProto()
		req := abci.RequestInitChain{
			Time:            h.genDoc.GenesisTime,
			ChainId:         h.genDoc.ChainID,
			InitialHeight:   h.genDoc.InitialHeight,
			ConsensusParams: &pbParams,
			Validators:      nextVals,
			AppStateBytes:   h.genDoc.AppState,
		}
		res, err := proxyApp.Consensus().InitChainSync(context.Background(), req)
		if err != nil {
			return nil, err
		}

		appHash = res.AppHash

		if stateBlockHeight == 0 { // we only update state when we are in initial state
			// If the app did not return an app hash, we keep the one set from the genesis doc in
			// the state. We don't set appHash since we don't want the genesis doc app hash
			// recorded in the genesis block. We should probably just remove GenesisDoc.AppHash.
			if len(res.AppHash) > 0 {
				state.AppHash = res.AppHash
			}
			// If the app returned validators or consensus params, update the state.
			if len(res.Validators) > 0 {
				vals, err := types.PB2TM.ValidatorUpdates(res.Validators)
				if err != nil {
					return nil, err
				}
				state.Validators = types.NewValidatorSet(vals)
				state.NextValidators = types.NewValidatorSet(vals).CopyIncrementProposerPriority(1)
			} else if len(h.genDoc.Validators) == 0 {
				// If validator set is not set in genesis and still empty after InitChain, exit.
				return nil, fmt.Errorf("validator set is nil in genesis and still empty after InitChain")
			}

			if res.ConsensusParams != nil {
				state.ConsensusParams = state.ConsensusParams.UpdateConsensusParams(res.ConsensusParams)
				state.Version.Consensus.App = state.ConsensusParams.Version.AppVersion
			}
			// We update the last results hash with the empty hash, to conform with RFC-6962.
			state.LastResultsHash = merkle.HashFromByteSlices(nil)
			if err := h.stateStore.Save(state); err != nil {
				return nil, err
			}
		}
	}

	// First handle edge cases and constraints on the storeBlockHeight and storeBlockBase.
	switch {
	case storeBlockHeight == 0:
		assertAppHashEqualsOneFromState(appHash, state)
		return appHash, nil

	case appBlockHeight == 0 && state.InitialHeight < storeBlockBase:
		// the app has no state, and the block store is truncated above the initial height
		return appHash, sm.ErrAppBlockHeightTooLow{AppHeight: appBlockHeight, StoreBase: storeBlockBase}

	case appBlockHeight > 0 && appBlockHeight < storeBlockBase-1:
		// the app is too far behind truncated store (can be 1 behind since we replay the next)
		return appHash, sm.ErrAppBlockHeightTooLow{AppHeight: appBlockHeight, StoreBase: storeBlockBase}

	case storeBlockHeight < appBlockHeight:
		// the app should never be ahead of the store (but this is under app's control)
		return appHash, sm.ErrAppBlockHeightTooHigh{CoreHeight: storeBlockHeight, AppHeight: appBlockHeight}

	case storeBlockHeight < stateBlockHeight:
		// the state should never be ahead of the store (this is under tendermint's control)
		panic(fmt.Sprintf("StateBlockHeight (%d) > StoreBlockHeight (%d)", stateBlockHeight, storeBlockHeight))

	case storeBlockHeight > stateBlockHeight+1:
		// store should be at most one ahead of the state (this is under tendermint's control)
		panic(fmt.Sprintf("StoreBlockHeight (%d) > StateBlockHeight + 1 (%d)", storeBlockHeight, stateBlockHeight+1))
	}

	var err error
	// Now either store is equal to state, or one ahead.
	// For each, consider all cases of where the app could be, given app <= store
	if storeBlockHeight == stateBlockHeight {
		// Tendermint ran Commit and saved the state.
		// Either the app is asking for replay, or we're all synced up.
		if appBlockHeight < storeBlockHeight {
			// the app is behind, so replay blocks, but no need to go through WAL (state is already synced to store)
			return h.replayBlocks(state, proxyApp, appBlockHeight, storeBlockHeight, false)

		} else if appBlockHeight == storeBlockHeight {
			// We're good!
			assertAppHashEqualsOneFromState(appHash, state)
			return appHash, nil
		}

	} else if storeBlockHeight == stateBlockHeight+1 {
		// We saved the block in the store but haven't updated the state,
		// so we'll need to replay a block using the WAL.
		switch {
		case appBlockHeight < stateBlockHeight:
			// the app is further behind than it should be, so replay blocks
			// but leave the last block to go through the WAL
			return h.replayBlocks(state, proxyApp, appBlockHeight, storeBlockHeight, true)

		case appBlockHeight == stateBlockHeight:
			// We haven't run Commit (both the state and app are one block behind),
			// so replayBlock with the real app.
			// NOTE: We could instead use the cs.WAL on cs.Start,
			// but we'd have to allow the WAL to replay a block that wrote it's #ENDHEIGHT
			h.logger.Info("Replay last block using real app")
			state, err = h.replayBlock(state, storeBlockHeight, proxyApp.Consensus())
			return state.AppHash, err

		case appBlockHeight == storeBlockHeight:
			// We ran Commit, but didn't save the state, so replayBlock with mock app.
			abciResponses, err := h.stateStore.LoadABCIResponses(storeBlockHeight)
			if err != nil {
				return nil, err
			}
			mockApp := newMockProxyApp(appHash, abciResponses)
			h.logger.Info("Replay last block using mock app")
			state, err = h.replayBlock(state, storeBlockHeight, mockApp)
			return state.AppHash, err
		}

	}

	panic(fmt.Sprintf("uncovered case! appHeight: %d, storeHeight: %d, stateHeight: %d",
		appBlockHeight, storeBlockHeight, stateBlockHeight))
}

func (h *Handshaker) replayBlocks(
	state sm.State,
	proxyApp proxy.AppConns,
	appBlockHeight,
	storeBlockHeight int64,
	mutateState bool) ([]byte, error) {
	// App is further behind than it should be, so we need to replay blocks.
	// We replay all blocks from appBlockHeight+1.
	//
	// Note that we don't have an old version of the state,
	// so we by-pass state validation/mutation using sm.ExecCommitBlock.
	// This also means we won't be saving validator sets if they change during this period.
	// TODO: Load the historical information to fix this and just use state.ApplyBlock
	//
	// If mutateState == true, the final block is replayed with h.replayBlock()

	var appHash []byte
	var err error
	finalBlock := storeBlockHeight
	if mutateState {
		finalBlock--
	}
	firstBlock := appBlockHeight + 1
	if firstBlock == 1 {
		firstBlock = state.InitialHeight
	}
	for i := firstBlock; i <= finalBlock; i++ {
		h.logger.Info("Applying block", "height", i)
		block := h.store.LoadBlock(i)
		// Extra check to ensure the app was not changed in a way it shouldn't have.
		if len(appHash) > 0 {
			assertAppHashEqualsOneFromBlock(appHash, block)
		}

		appHash, err = sm.ExecCommitBlock(proxyApp.Consensus(), block, h.logger, h.stateStore, h.genDoc.InitialHeight)
		if err != nil {
			return nil, err
		}

		h.nBlocks++
	}

	if mutateState {
		// sync the final block
		state, err = h.replayBlock(state, storeBlockHeight, proxyApp.Consensus())
		if err != nil {
			return nil, err
		}
		appHash = state.AppHash
	}

	assertAppHashEqualsOneFromState(appHash, state)
	return appHash, nil
}

// ApplyBlock on the proxyApp with the last block.
func (h *Handshaker) replayBlock(state sm.State, height int64, proxyApp proxy.AppConnConsensus) (sm.State, error) {
	block := h.store.LoadBlock(height)
	meta := h.store.LoadBlockMeta(height)

	// Use stubs for both mempool and evidence pool since no transactions nor
	// evidence are needed here - block already exists.
	blockExec := sm.NewBlockExecutor(h.stateStore, h.logger, proxyApp, emptyMempool{}, sm.EmptyEvidencePool{})
	blockExec.SetEventBus(h.eventBus)

	var err error
	state, _, err = blockExec.ApplyBlock(state, meta.BlockID, block)
	if err != nil {
		return sm.State{}, err
	}

	h.nBlocks++

	return state, nil
}

func assertAppHashEqualsOneFromBlock(appHash []byte, block *types.Block) {
	if !bytes.Equal(appHash, block.AppHash) {
		panic(fmt.Sprintf(`block.AppHash does not match AppHash after replay. Got %X, expected %X.

Block: %v
`,
			appHash, block.AppHash, block))
	}
}

func assertAppHashEqualsOneFromState(appHash []byte, state sm.State) {
	if !bytes.Equal(appHash, state.AppHash) {
		panic(fmt.Sprintf(`state.AppHash does not match AppHash after replay. Got
%X, expected %X.

State: %v

Did you reset Tendermint without resetting your application's data?`,
			appHash, state.AppHash, state))
	}
}
