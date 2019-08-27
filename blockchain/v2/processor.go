package v2

import (
	"fmt"

	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

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

type processBlock struct {
}

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

type pcStop struct{}
type pcFinished struct {
	height int64
}
type scFinished struct{}
type pcWaitingForBlock struct{}

func (p pcFinished) Error() string {
	return "finished"
}

// * accept the initial state from the blockchain reactior initialization
// * produce the final state from `SwitchToConsensus`
func newProcessor(initBlock *types.Block, state state.State, chainID string, context processorContext) *Routine {
	bq := newBlockQueue(initBlock)
	draining := false
	handlerFunc := func(event Event) (Event, error) {
		switch event := event.(type) {
		case *bcBlockResponse:
			err := bq.add(event.peerID, event.block, event.height)
			if err != nil {
				return pcDuplicateBlock{}, nil
			}
			return processBlock{}, nil
		case processBlock:
			firstItem, secondItem, err := bq.nextTwo()
			if err != nil {
				return pcWaitingForBlock{}, nil
			}
			first, second := firstItem.block, secondItem.block

			firstParts := first.MakePartSet(types.BlockPartSizeBytes)
			firstPartsHeader := firstParts.Header()
			firstID := types.BlockID{Hash: first.Hash(), PartsHeader: firstPartsHeader}

			err = context.verifyCommit(chainID, firstID, first.Height, second.LastCommit)
			if err != nil {
				// XXX: maybe add peer and height fiields
				return pcBlockVerificationFailure{}, nil
			}

			context.saveBlock(first, firstParts, second.LastCommit)

			state, err = context.applyBlock(state, firstID, first)
			if err != nil {
				panic(fmt.Sprintf("failed to process committed block (%d:%X): %v", first.Height, first.Hash(), err))
			}
			bq.advance()
			return pcBlockProcessed{first.Height, firstItem.peerID}, nil
		case *peerError:
			bq.remove(event.peerID)
			return NoOp{}, nil
		case pcBlockProcessed:
			if bq.empty() {
				return NoOp{}, pcFinished{height: bq.height}
			}
			return processBlock{}, nil
		case pcStop:
			if bq.empty() {
				return NoOp{}, pcFinished{height: bq.height}
			}
			draining = true
			return processBlock{}, nil
		case pcWaitingForBlock:
			if draining {
				return NoOp{}, pcFinished{height: bq.height}
			}
			return NoOp{}, nil
		}

		return NoOp{}, nil
	}

	return newRoutine("processor", handlerFunc)
}
