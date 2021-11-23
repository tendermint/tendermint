package lockedvalue

import (
	"time"

	"github.com/ds-test-framework/scheduler/log"
	"github.com/ds-test-framework/scheduler/testlib"
	smlib "github.com/ds-test-framework/scheduler/testlib/statemachine"
	"github.com/ds-test-framework/scheduler/types"
	"github.com/ds-test-framework/tendermint-test/util"
)

type testCaseTwoFilters struct{}

func (testCaseTwoFilters) Round2(c *smlib.Context) ([]*types.Message, bool) {

	tMsg, err := util.GetMessageFromSendEvent(c.CurEvent, c.MessagePool)
	if err != nil {
		return []*types.Message{}, false
	}
	_, round := util.ExtractHR(tMsg)
	if round == -1 {
		return []*types.Message{tMsg.SchedulerMessage}, true
	}
	if round != 2 {
		return []*types.Message{}, false
	}

	switch tMsg.Type {
	case util.Proposal:
		blockID, ok := util.GetProposalBlockIDS(tMsg)
		if ok {
			c.Vars.Set("newProposal", blockID)
		}
	case util.Prevote:
		partition := getReplicaPartition(c.Context)
		honestDelayed, _ := partition.GetPart("honestDelayed")
		replica, _ := c.Replicas.Get(tMsg.From)

		if honestDelayed.Contains(tMsg.From) && util.IsVoteFrom(tMsg, replica) {
			// c.Logger().Info("Checking unlocked vote")
			newProp, _ := c.Vars.GetString("newProposal")
			voteBlockID, ok := util.GetVoteBlockIDS(tMsg)
			c.Vars.Set("vote", voteBlockID)
			if ok && voteBlockID == newProp {
				c.Logger().With(log.LogParams{
					"round1_proposal": newProp,
					"vote":            voteBlockID,
				}).Info("Failing because replica did unlocked")
				c.Abort()
			}
		}
	}
	return []*types.Message{tMsg.SchedulerMessage}, true
}

func Two() *testlib.TestCase {

	commonCond := commonCond{}
	tOneCond := testCaseOneCond{}
	tOneFilters := testCaseOneFilters{}
	filters := testCaseTwoFilters{}

	stateMachine := smlib.NewStateMachine()
	builder := stateMachine.Builder()
	round2 := builder.
		On(commonCond.valueLockedCond, "LockedValue").
		On(commonCond.roundReached(1), "Round1").
		On(commonCond.roundReached(2), "Round2")

	round2.On(tOneCond.commitNewCond, smlib.SuccessStateLabel)
	round2.On(tOneCond.commitOldCond, smlib.FailStateLabel)

	handler := smlib.NewAsyncStateMachineHandler(stateMachine)
	handler.AddEventHandler(tOneFilters.faultyReplicaFilter)
	handler.AddEventHandler(tOneFilters.Round0)
	handler.AddEventHandler(tOneFilters.Round1)
	handler.AddEventHandler(filters.Round2)

	testcase := testlib.NewTestCase("LockedValueOne", 50*time.Second, handler)
	testcase.SetupFunc(testCaseOneSetup)

	testcase.AssertFn(func(c *testlib.Context) bool {
		oldProposal, ok := c.Vars.GetString("oldProposal")
		if !ok {
			return false
		}
		vote, ok := c.Vars.GetString("vote")
		if !ok {
			return false
		}
		return vote == oldProposal
	})

	return testcase
}
