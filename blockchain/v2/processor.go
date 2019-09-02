package v2

import (
	"fmt"

	"github.com/tendermint/tendermint/p2p"
	bcState "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

type peerError struct {
	peerID p2p.ID
}

type queueItem struct {
	block  *types.Block
	peerID p2p.ID
}

type blockQueue struct {
	queue  map[int64]*queueItem
	height int64 // Height is the last validated block
}

// we initialize the block queue with block stored on the node
func newBlockQueue(initBlock *types.Block) *blockQueue {
	var myID p2p.ID = "thispeersid" // XXX: make this real
	initItem := &queueItem{block: initBlock, peerID: myID}

	return &blockQueue{
		queue:  map[int64]*queueItem{initBlock.Height: initItem},
		height: initBlock.Height,
	}
}

// nextTwo returns the next two unverified blocks
func (bq *blockQueue) nextTwo() (*queueItem, *queueItem, error) {
	if first, ok := bq.queue[bq.height+1]; ok {
		if second, ok := bq.queue[bq.height+2]; ok {
			return first, second, nil
		}
	}
	return nil, nil, fmt.Errorf("not found")
}

func (bq *blockQueue) empty() bool {
	return len(bq.queue) == 0
}

func (bq *blockQueue) advance() {
	delete(bq.queue, bq.height)
	bq.height++
}

func (bq *blockQueue) add(peerID p2p.ID, block *types.Block, height int64) error {
	if _, ok := bq.queue[height]; ok {
		return fmt.Errorf("duplicate queue item")
	}
	bq.queue[height] = &queueItem{block: block, peerID: peerID}
	return nil
}

func (bq *blockQueue) remove(peerID p2p.ID) {
	for height, item := range bq.queue {
		if item.peerID == peerID {
			delete(bq.queue, height)
		}
	}
}

type pcDuplicateBlock struct{}

type bcBlockResponse struct {
	peerID p2p.ID
	block  *types.Block
	height int64
}

type pcBlockVerificationFailure struct{}

type pcBlockProcessed struct {
	height int64
	peerID p2p.ID
}

type pcProcessBlock struct{}

type pcStop struct{}

type pcFinished struct {
	height       int64
	blocksSynced int
	state        bcState.State
}

func (p pcFinished) Error() string {
	return "finished"
}

var noOp struct{} = struct{}{}

// TODO: timeouts

// XXX: the last error doesn't need to be an error just a termination
// XXX: Maybe merge this with the blockQueue structure
type pcState struct {
	chainID      string
	blocksSynced int
	draining     bool
	processing   bool
	bq           *blockQueue
	bcState      bcState.State
	context      processorContext
	rounds       int
}

func (ps *pcState) String() string {
	return fmt.Sprintf("queue: %d draining: %v, processing: %v\n", len(ps.bq.queue), ps.draining, ps.processing)
}

func handleFSM(event Event, state *pcState) (Event, *pcState, error) {
	switch event := event.(type) {
	case *bcBlockResponse:
		if state.rounds > 0 {
			state.rounds--
		}
		err := state.bq.add(event.peerID, event.block, event.height)
		if err != nil {
			return pcDuplicateBlock{}, state, nil
		}

		if !state.processing {
			state.processing = true
			return pcProcessBlock{}, state, nil
		}
	case pcProcessBlock:
		if state.rounds > 0 {
			state.rounds--
		}
		state.processing = false
		firstItem, secondItem, err := state.bq.nextTwo()
		if err != nil {
			if state.draining {
				return noOp, state, pcFinished{height: state.bq.height}
			}
			return noOp, state, nil
		}
		first, second := firstItem.block, secondItem.block

		firstParts := first.MakePartSet(types.BlockPartSizeBytes)
		firstPartsHeader := firstParts.Header()
		firstID := types.BlockID{Hash: first.Hash(), PartsHeader: firstPartsHeader}

		err = state.context.verifyCommit(state.chainID, firstID, first.Height, second.LastCommit)
		if err != nil {
			// XXX: maybe add peer and height fiields
			return pcBlockVerificationFailure{}, state, nil
		}

		state.context.saveBlock(first, firstParts, second.LastCommit)

		state.bcState, err = state.context.applyBlock(state.bcState, firstID, first)
		if err != nil {
			panic(fmt.Sprintf("failed to process committed block (%d:%X): %v", first.Height, first.Hash(), err))
		}
		state.bq.advance()
		state.blocksSynced++
		return pcBlockProcessed{first.Height, firstItem.peerID}, state, nil
	case *peerError:
		state.bq.remove(event.peerID)
	case pcBlockProcessed:
		if state.rounds > 0 {
			state.rounds--
		}
		if state.bq.empty() {
			return noOp, state, pcFinished{height: state.bq.height, blocksSynced: state.blocksSynced}
		}
		if !state.processing {
			state.processing = true
			return pcProcessBlock{}, state, nil
		}
	case pcStop:
		if state.bq.empty() {
			return noOp, state, pcFinished{height: state.bq.height, blocksSynced: state.blocksSynced}
		}
		state.draining = true
		if !state.processing {
			state.processing = true
			return pcProcessBlock{}, state, nil
		}
	}
	state.rounds++
	if state.rounds > 5 {
		return noOp, state, fmt.Errorf("lack of progress")
	}

	return noOp, state, nil
}

func newProcessor(initBlock *types.Block, bcState bcState.State, chainID string, context processorContext) *Routine {
	state := &pcState{
		bq:           newBlockQueue(initBlock),
		draining:     false,
		processing:   false,
		blocksSynced: 0,
		context:      context,
		bcState:      bcState,
	}

	handlerFunc := func(event Event) (Event, error) {
		event, nextState, err := handleFSM(event, state)
		state = nextState
		return event, err
	}

	return newRoutine("processor", handlerFunc)
}
