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
	height int64
	peerID p2p.ID
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
	height int64
}

func newBlockQueue(initHeight int64) *blockQueue {
	return &blockQueue{
		queue:  map[int64]*queueItem{},
		height: initHeight,
	}
}

func (bq *blockQueue) nextTwo() (*queueItem, *queueItem, error) {
	if first, ok := bq.queue[bq.height]; ok {
		if second, ok := bq.queue[bq.height+1]; ok {
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
type pcFinished struct{}
type scFinished struct{}

func (p pcFinished) Error() string {
	return "finished"
}

// * accept the initial state from the blockchain reactior initialization
// * produce the final state from `SwitchToConsensus`
func newProcessor(initHeight int64, state state.State, chainID string, context processorContext) *Routine {
	bq := newBlockQueue(initHeight)
	draining := false
	handlerFunc := func(event Event) (Events, error) {
		switch event := event.(type) {
		case *bcBlockResponse:
			err := bq.add(event.peerID, event.block, event.height)
			if err != nil {
				return Events{pcDuplicateBlock{}}, nil
			}
			return Events{processBlock{}}, nil
		case *processBlock:
			firstItem, secondItem, err := bq.nextTwo()
			if err != nil {
				// We need both to sync the first block.
				return Events{}, nil
			}
			first, second := firstItem.block, secondItem.block

			firstParts := first.MakePartSet(types.BlockPartSizeBytes)
			firstPartsHeader := firstParts.Header()
			firstID := types.BlockID{Hash: first.Hash(), PartsHeader: firstPartsHeader}

			err = context.verifyCommit(chainID, firstID, first.Height, second.LastCommit)
			if err != nil {
				return Events{pcBlockVerificationFailure{}}, nil
			}

			context.saveBlock(first, firstParts, second.LastCommit)

			state, err = context.applyBlock(state, firstID, first)
			if err != nil {
				panic(fmt.Sprintf("failed to process committed block (%d:%X): %v", first.Height, first.Hash(), err))
			}
			bq.advance()
			if bq.empty() && draining {
				return Events{pcBlockProcessed{event.height, event.peerID}, pcStop{}}, nil
			} else {
				return Events{processBlock{}, pcBlockProcessed{event.height, event.peerID}}, nil
			}
		case *peerError:
			bq.remove(event.peerID)
			return Events{}, nil
		case *scFinished:
			draining = true
			return Events{processBlock{}}, nil
		case pcStop:
			return Events{}, pcFinished{}
		}

		return Events{}, nil
	}

	return newRoutine("processor", handlerFunc)
}
