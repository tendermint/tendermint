package sanity

import (
	"time"

	"github.com/ds-test-framework/scheduler/log"
	"github.com/ds-test-framework/scheduler/testlib"
	smlib "github.com/ds-test-framework/scheduler/testlib/statemachine"
	"github.com/ds-test-framework/scheduler/types"
	"github.com/ds-test-framework/tendermint-test/util"
)

type threeFilters struct{}

func (threeFilters) round0(c *smlib.Context) ([]*types.Message, bool) {
	tMsg, err := util.GetMessageFromSendEvent(c.CurEvent, c.MessagePool)
	if err != nil {
		return []*types.Message{}, false
	}

	_, round := util.ExtractHR(tMsg)
	if round == -1 {
		return []*types.Message{tMsg.SchedulerMessage}, true
	}
	if round != 0 {
		return []*types.Message{}, false
	}
	partition := getPartition(c.Context)
	toNotLock, _ := partition.GetPart("toNotLock")

	switch {
	case tMsg.Type == util.Prevote && toNotLock.Contains(tMsg.To):
		voteCount := getVoteCount(c.Context)
		faults, _ := c.Vars.GetInt("faults")
		_, ok := voteCount[string(tMsg.To)]
		if !ok {
			voteCount[string(tMsg.To)] = 0
		}
		curCount := voteCount[string(tMsg.To)]
		if curCount >= 2*faults-1 {
			return []*types.Message{}, true
		}
		voteCount[string(tMsg.To)] = curCount + 1
		c.Vars.Set("voteCount", voteCount)
		return []*types.Message{tMsg.SchedulerMessage}, true
	case tMsg.Type == util.Proposal && round == 0:
		blockID, ok := util.GetProposalBlockIDS(tMsg)
		if ok {
			c.Vars.Set("oldProposal", blockID)
		}
	}

	return []*types.Message{tMsg.SchedulerMessage}, true
}

func (threeFilters) round1(c *smlib.Context) ([]*types.Message, bool) {
	tMsg, err := util.GetMessageFromSendEvent(c.CurEvent, c.MessagePool)
	if err != nil {
		return []*types.Message{}, false
	}

	_, round := util.ExtractHR(tMsg)
	if round == -1 {
		return []*types.Message{tMsg.SchedulerMessage}, true
	}
	if round != 1 {
		return []*types.Message{}, false
	}
	if tMsg.Type == util.Proposal {
		return []*types.Message{}, true
	}

	return []*types.Message{tMsg.SchedulerMessage}, true
}

func threeSetup(c *testlib.Context) error {
	faults := int((c.Replicas.Cap() - 1) / 3)
	partitioner := util.NewGenericParititioner(c.Replicas)
	partition, err := partitioner.CreateParition([]int{faults + 1, 2*faults - 1, 1}, []string{"toLock", "toNotLock", "faulty"})
	if err != nil {
		return err
	}
	c.Logger().With(log.LogParams{
		"partition": partition.String(),
	}).Info("Created partition")
	c.Vars.Set("partition", partition)
	c.Vars.Set("voteCount", make(map[string]int))
	c.Vars.Set("faults", faults)
	c.Vars.Set("roundCount", make(map[string]int))
	return nil
}

type threeCond struct{}

func (threeCond) commit(c *smlib.Context) bool {
	cEventType := c.CurEvent.Type
	switch cEventType := cEventType.(type) {
	case *types.GenericEventType:
		if cEventType.T != "Committing block" {
			return false
		}
		blockID, ok := cEventType.Params["block_id"]
		if !ok {
			return false
		}

		oldProposalI, ok := c.Vars.Get("oldProposal")
		if !ok {
			return false
		}
		oldProposal := oldProposalI.(string)
		c.Logger().With(log.LogParams{
			"old_proposal": oldProposal,
			"commit_block": blockID,
		}).Info("Checking commit")
		if blockID == oldProposal {
			c.Vars.Set("CommitBlockID", blockID)
			return true
		}
	}
	return false
}

func ThreeTestCase() *testlib.TestCase {

	commonFilters := commonFilters{}
	commonCond := commonCond{}
	filters := threeFilters{}
	cond := threeCond{}

	sm := smlib.NewStateMachine()
	start := sm.Builder()
	start.On(commonCond.commitCond, smlib.FailStateLabel)
	round1 := start.On(commonCond.roundReached(1), "round1")
	round1.On(commonCond.commitCond, smlib.FailStateLabel)
	round2 := round1.On(commonCond.roundReached(2), "round2")
	round2.On(cond.commit, smlib.SuccessStateLabel)

	handler := smlib.NewAsyncStateMachineHandler(sm)
	handler.AddEventHandler(commonFilters.faultyFilter)
	handler.AddEventHandler(filters.round0)
	handler.AddEventHandler(filters.round1)

	testcase := testlib.NewTestCase("LockedValueCheck", 30*time.Second, handler)
	testcase.SetupFunc(threeSetup)
	testcase.AssertFn(func(c *testlib.Context) bool {
		commitBlock, ok := c.Vars.GetString("CommitBlockID")
		if !ok {
			return false
		}
		oldProposal, ok := c.Vars.GetString("oldProposal")
		if !ok {
			return false
		}
		curRound, ok := c.Vars.GetInt("CurRound")
		if !ok {
			return false
		}
		c.Logger().With(log.LogParams{
			"commit_blockID": commitBlock,
			"proposal":       oldProposal,
			"curRound":       curRound,
		}).Info("Checking assertion")
		return commitBlock == oldProposal && curRound == 2
	})

	return testcase
}
