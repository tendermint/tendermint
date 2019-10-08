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
	peerID p2p.ID
	height int64
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
	blocksSynced int64
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
	blocksSynced int64
	draining     bool
	tdState      tdState.State
	context      processorContext
}

func (pcFSM *pcState) String() string {
	return fmt.Sprintf("height: %d queue length: %d draining: %v blocks synced: %d",
		pcFSM.height, len(pcFSM.queue), pcFSM.draining, pcFSM.blocksSynced)
}

// newPcState returns a pcState initialized with the last verified block enqueued
func newPcState(initHeight int64, tdState tdState.State, chainID string, context processorContext) *pcState {
	return &pcState{
		height:       initHeight,
		queue:        blockQueue{},
		chainID:      chainID,
		draining:     false,
		blocksSynced: 0,
		context:      context,
		tdState:      tdState,
	}
}

// nextTwo returns the next two unverified blocks
func (pcFSM *pcState) nextTwo() (queueItem, queueItem, error) {
	if first, ok := pcFSM.queue[pcFSM.height+1]; ok {
		if second, ok := pcFSM.queue[pcFSM.height+2]; ok {
			return first, second, nil
		}
	}
	return queueItem{}, queueItem{}, fmt.Errorf("not found")
}

// synced returns true when at most the last verified block remains in the queue
func (pcFSM *pcState) synced() bool {
	return len(pcFSM.queue) <= 1
}

func (pcFSM *pcState) advance() {
	pcFSM.height++
	delete(pcFSM.queue, pcFSM.height)
	pcFSM.blocksSynced++
}

func (pcFSM *pcState) enqueue(peerID p2p.ID, block *types.Block, height int64) error {
	if _, ok := pcFSM.queue[height]; ok {
		return fmt.Errorf("duplicate queue item")
	}
	pcFSM.queue[height] = queueItem{block: block, peerID: peerID}
	return nil
}

// purgePeer moves all unprocessed blocks from the queue
func (pcFSM *pcState) purgePeer(peerID p2p.ID) {
	// what if height is less than pcFSM.height?
	for height, item := range pcFSM.queue {
		if item.peerID == peerID {
			delete(pcFSM.queue, height)
		}
	}
}

// handle processes FSM events
func (pcFSM *pcState) handle(event Event) (Event, error) {
	switch event := event.(type) {
	case *bcBlockResponse:
		err := pcFSM.enqueue(event.peerID, event.block, event.height)
		if err != nil {
			return pcDuplicateBlock{}, nil
		}

	case pcProcessBlock:
		firstItem, secondItem, err := pcFSM.nextTwo()
		if err != nil {
			if pcFSM.draining {
				return noOp, pcFinished{height: pcFSM.height}
			}
			return noOp, nil
		}
		first, second := firstItem.block, secondItem.block

		firstParts := first.MakePartSet(types.BlockPartSizeBytes)
		firstPartsHeader := firstParts.Header()
		firstID := types.BlockID{Hash: first.Hash(), PartsHeader: firstPartsHeader}

		err = pcFSM.context.verifyCommit(pcFSM.chainID, firstID, first.Height, second.LastCommit)
		if err != nil {
			return pcBlockVerificationFailure{peerID: firstItem.peerID, height: first.Height}, nil
		}

		pcFSM.context.saveBlock(first, firstParts, second.LastCommit)

		pcFSM.tdState, err = pcFSM.context.applyBlock(pcFSM.tdState, firstID, first)
		if err != nil {
			panic(fmt.Sprintf("failed to process committed block (%d:%X): %v", first.Height, first.Hash(), err))
		}
		pcFSM.advance()
		return pcBlockProcessed{height: first.Height, peerID: firstItem.peerID}, nil

	case *peerError:
		pcFSM.purgePeer(event.peerID)

	case pcStop:
		if pcFSM.synced() {
			return noOp, pcFinished{height: pcFSM.height, blocksSynced: pcFSM.blocksSynced}
		}
		pcFSM.draining = true
	}

	return noOp, nil
}

func newProcessor(pcFSM *pcState, queueSize int) *Routine {
	return newRoutine("processor", pcFSM.handle, queueSize)
}
