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
type pcFinished struct {
	height int64
}
type scFinished struct{}

func (p pcFinished) Error() string {
	return "finished"
}

// XXX: this assumes that feedback will be in play here

// * accept the initial state from the blockchain reactior initialization
// * produce the final state from `SwitchToConsensus`
func newProcessor(initHeight int64, state state.State, chainID string, context processorContext) *Routine {
	bq := newBlockQueue(initHeight)
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
			fmt.Println("processing block")
			firstItem, secondItem, err := bq.nextTwo()
			if err != nil {
				// We need both to sync the first block.
				return NoOp{}, nil
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
			if bq.empty() && draining {
				return NoOp{}, pcFinished{height: bq.height}
			}
			return processBlock{}, nil
		case pcStop:
			if bq.empty() {
				return NoOp{}, pcFinished{height: bq.height}
			}
			draining = true
			return processBlock{}, nil
		case NoOp:
			return NoOp{}, fmt.Errorf("failed")
		}

		return NoOp{}, nil
	}

	return newRoutine("processor", handlerFunc)
}
