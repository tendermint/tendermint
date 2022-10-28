package state

import (
	"encoding/hex"
	"github.com/tendermint/tendermint/libs/log"
	tmstate "github.com/tendermint/tendermint/proto/tendermint/state"
	"github.com/tendermint/tendermint/proxy"
	"github.com/tendermint/tendermint/types"
)

type executionResult struct {
	res *tmstate.ABCIResponses
	err error
}

type executionTask struct {
	height         int64
	index          int64
	block          *types.Block
	stopped        bool
	taskResultChan chan *executionTask
	result         *executionResult
	proxyApp       proxy.AppConnConsensus
	store          Store
	logger         log.Logger
	blockHash      string
	initialHeight  int64
}

func newExecutionTask(blockExec *BlockExecutor, block *types.Block, index int64) *executionTask {
	ret := &executionTask{
		height:         block.Height,
		block:          block,
		store:          blockExec.store,
		proxyApp:       blockExec.proxyApp,
		logger:         blockExec.logger,
		taskResultChan: blockExec.prerunCtx.taskResultChan,
		index:          index,
	}
	ret.blockHash = hex.EncodeToString(block.Hash())

	return ret
}

func (e *executionTask) dump(when string) {
	e.logger.Info(when,
		"stopped", e.stopped,
		"Height", e.block.Height,
		"index", e.index,
		"blockHash", e.blockHash,
		//"AppHash", e.block.AppHash,
	)
}

func (t *executionTask) stop() {
	if t.stopped {
		return
	}

	t.stopped = true
}

func (t *executionTask) run() {
	t.dump("Start prerun")

	abciResponses, err := execBlockOnProxyApp(
		t.logger, t.proxyApp, t.block, t.store, t.initialHeight,
	)

	if !t.stopped {
		t.result = &executionResult{
			abciResponses, err,
		}
	}
	t.dump("Prerun completed")
	t.taskResultChan <- t
}

//========================================================
func (blockExec *BlockExecutor) InitPrerun() {
	go blockExec.prerunCtx.prerunRoutine()
}

func (blockExec *BlockExecutor) NotifyPrerun(block *types.Block) {
	blockExec.prerunCtx.notifyPrerun(blockExec, block)
}
