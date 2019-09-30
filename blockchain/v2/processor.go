package v2

import (
	"fmt"

	"github.com/tendermint/tendermint/p2p"
	tdState "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

type peerError struct {
	priorityHigh
	peerID p2p.ID
}

type pcDuplicateBlock struct {
	priorityNormal
}

type bcBlockResponse struct {
	priorityNormal
	peerID p2p.ID
	block  *types.Block
	height int64
}

type pcBlockVerificationFailure struct {
	priorityNormal
}

type pcBlockProcessed struct {
	priorityNormal
	height int64
	peerID p2p.ID
}

type pcProcessBlock struct {
	priorityNormal
}

type pcStop struct {
	priorityNormal
}

type pcFinished struct {
	priorityNormal
	height       int64
	blocksSynced int
	state        tdState.State
}

func (p pcFinished) Error() string {
	return "finished"
}

type queueItem struct {
	block  *types.Block
	peerID p2p.ID
}

type blockQueue map[int64]queueItem

type pcState struct {
	height       int64
	queue        blockQueue
	chainID      string
	blocksSynced int
	draining     bool
	tdState      tdState.State
	context      processorContext
}

func (ps *pcState) String() string {
	return fmt.Sprintf("queue: %d draining: %v, \n", len(ps.queue), ps.draining)
}

// newPcState returns a pcState initialized with the last verified block enqueued
func newPcState(initBlock *types.Block, tdState tdState.State, chainID string, context processorContext) *pcState {
	var myID p2p.ID = "thispeersid" // XXX: this should be the nodes peerID
	initItem := queueItem{block: initBlock, peerID: myID}

	return &pcState{
		height:       initBlock.Height,
		queue:        blockQueue{initBlock.Height: initItem},
		chainID:      chainID,
		draining:     false,
		blocksSynced: 0,
		context:      context,
		tdState:      tdState,
	}
}

// nextTwo returns the next two unverified blocks
func (state *pcState) nextTwo() (queueItem, queueItem, error) {
	if first, ok := state.queue[state.height+1]; ok {
		if second, ok := state.queue[state.height+2]; ok {
			return first, second, nil
		}
	}
	return queueItem{}, queueItem{}, fmt.Errorf("not found")
}

// synced returns true when only the last verified block remains in the queue
func (state *pcState) synced() bool {
	return len(state.queue) == 1
}

func (state *pcState) advance() {
	delete(state.queue, state.height)
	state.height++
	state.blocksSynced++
}

func (state *pcState) enqueue(peerID p2p.ID, block *types.Block, height int64) error {
	if _, ok := state.queue[height]; ok {
		return fmt.Errorf("duplicate queue item")
	}
	state.queue[height] = queueItem{block: block, peerID: peerID}
	return nil
}

// XXX:  rename pcFSM
// purgePeer moves all unprocessed blocks from the queue
func (state *pcState) purgePeer(peerID p2p.ID) {
	// what if height is less than state.height?
	for height, item := range state.queue {
		if item.peerID == peerID {
			delete(state.queue, height)
		}
	}
}

// Handle
func (state *pcState) handle(event Event) (Event, error) {
	switch event := event.(type) {
	case *bcBlockResponse:
		err := state.enqueue(event.peerID, event.block, event.height)
		if err != nil {
			return pcDuplicateBlock{}, nil
		}

	case pcProcessBlock:
		firstItem, secondItem, err := state.nextTwo()
		if err != nil {
			if state.draining {
				return noOp, pcFinished{height: state.height}
			}
			return noOp, nil
		}
		first, second := firstItem.block, secondItem.block

		firstParts := first.MakePartSet(types.BlockPartSizeBytes)
		firstPartsHeader := firstParts.Header()
		firstID := types.BlockID{Hash: first.Hash(), PartsHeader: firstPartsHeader}

		err = state.context.verifyCommit(state.chainID, firstID, first.Height, second.LastCommit)
		if err != nil {
			// XXX: maybe add peer and height field
			return pcBlockVerificationFailure{}, nil
		}

		state.context.saveBlock(first, firstParts, second.LastCommit)

		state.tdState, err = state.context.applyBlock(state.tdState, firstID, first)
		if err != nil {
			panic(fmt.Sprintf("failed to process committed block (%d:%X): %v", first.Height, first.Hash(), err))
		}
		state.advance()
		return pcBlockProcessed{height: first.Height, peerID: firstItem.peerID}, nil
	case *peerError:
		state.purgePeer(event.peerID)
		// TODO: emit an event notifying the scheduler
	case pcStop:
		if state.synced() {
			return noOp, pcFinished{height: state.height, blocksSynced: state.blocksSynced}
		}
		state.draining = true
	}

	return noOp, nil
}

func newProcessor(state *pcState, queueSize int) *Routine {
	return newRoutine("processor", state.handle, queueSize)
}
