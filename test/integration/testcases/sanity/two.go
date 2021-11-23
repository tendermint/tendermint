package sanity

import (
	"time"

	"github.com/ds-test-framework/scheduler/log"
	"github.com/ds-test-framework/scheduler/testlib"
	smlib "github.com/ds-test-framework/scheduler/testlib/statemachine"
	"github.com/ds-test-framework/scheduler/types"
	"github.com/ds-test-framework/tendermint-test/util"
)

type twoFilters struct{}

func (twoFilters) round0(c *smlib.Context) ([]*types.Message, bool) {
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

	if tMsg.Type == util.Precommit {
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
	}

	return []*types.Message{tMsg.SchedulerMessage}, true
}

func (twoFilters) round1(c *smlib.Context) ([]*types.Message, bool) {
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
		replica, _ := c.Replicas.Get(tMsg.From)
		newProp, err := util.ChangeProposalBlockID(replica, tMsg)
		if err != nil {
			c.Logger().With(log.LogParams{"error": err}).Error("Failed to change proposal")
			return []*types.Message{tMsg.SchedulerMessage}, true
		}
		newMsgB, err := util.Marshal(newProp)
		if err != nil {
			c.Logger().With(log.LogParams{"error": err}).Error("Failed to marshal changed proposal")
			return []*types.Message{tMsg.SchedulerMessage}, true
		}
		return []*types.Message{c.NewMessage(tMsg.SchedulerMessage, newMsgB)}, true
	}
	return []*types.Message{tMsg.SchedulerMessage}, true
}

func getVoteCount(c *testlib.Context) map[string]int {
	v, _ := c.Vars.Get("voteCount")
	return v.(map[string]int)
}

func twoSetup(c *testlib.Context) error {
	faults := int((c.Replicas.Cap() - 1) / 3)
	partitioner := util.NewStaticPartitioner(c.Replicas, faults)
	partitioner.NewPartition(0)
	partition, _ := partitioner.GetPartition(0)
	c.Vars.Set("partition", partition)
	c.Vars.Set("faults", faults)
	c.Vars.Set("voteCount", make(map[string]int))
	c.Vars.Set("roundCount", make(map[string]int))
	return nil
}

// States:
// 	1. Ensure replicas skip round by not delivering enough precommits
//		1.1 One replica prevotes and precommits nil
// 	2. In the next round change the proposal block value
// 	3. Replicas should prevote and precommit the earlier block and commit
func TwoTestCase() *testlib.TestCase {

	filters := twoFilters{}
	commonFilters := commonFilters{}
	cond := commonCond{}

	sm := smlib.NewStateMachine()
	start := sm.Builder()
	start.On(cond.commitCond, smlib.FailStateLabel)
	round1 := start.On(cond.roundReached(1), "round1")
	round1.On(cond.commitCond, smlib.SuccessStateLabel)
	round1.On(cond.roundReached(2), smlib.FailStateLabel)

	handler := smlib.NewAsyncStateMachineHandler(sm)
	handler.AddEventHandler(commonFilters.faultyFilter)
	handler.AddEventHandler(filters.round0)
	handler.AddEventHandler(filters.round1)

	testcase := testlib.NewTestCase("WrongProposal", 30*time.Second, handler)
	testcase.SetupFunc(twoSetup)
	testcase.AssertFn(func(c *testlib.Context) bool {
		committed, ok := c.Vars.GetBool("Committed")
		if !ok {
			return false
		}
		curRound, ok := c.Vars.GetInt("CurRound")
		if !ok {
			return false
		}
		c.Logger().With(log.LogParams{
			"committed": committed,
			"cur_round": curRound,
		}).Info("Checking assertion")
		return curRound == 1 && committed
	})

	return testcase
}
