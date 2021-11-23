package lockedvalue

import (
	"time"

	"github.com/ds-test-framework/scheduler/log"
	"github.com/ds-test-framework/scheduler/testlib"
	smlib "github.com/ds-test-framework/scheduler/testlib/statemachine"
	"github.com/ds-test-framework/scheduler/types"
	"github.com/ds-test-framework/tendermint-test/util"

	ttypes "github.com/tendermint/tendermint/types"
)

var (
	stateLockedValue = "lockedValue"
	stateRound1      = "round1"
	stateForceRelock = "forceRelock"
	// stateRelocked    = "relocked"
)

type testCaseThreeFilters struct{}

func (testCaseThreeFilters) faultyVoteFilter(c *smlib.Context) ([]*types.Message, bool) {
	tMsg, err := util.GetMessageFromSendEvent(c.CurEvent, c.MessagePool)
	if err != nil {
		return []*types.Message{}, false
	}
	if tMsg.Type != util.Precommit && tMsg.Type != util.Prevote {
		return []*types.Message{}, false
	}

	partition := getReplicaPartition(c.Context)
	faulty, _ := partition.GetPart("faulty")
	honestDelayed, _ := partition.GetPart("honestDelayed")
	faultyReplica, _ := c.Replicas.Get(tMsg.From)
	if !faulty.Contains(tMsg.From) {
		return []*types.Message{}, false
	}

	if c.StateMachine.CurState().Is(stateForceRelock) {
		if honestDelayed.Contains(tMsg.To) {
			newPropBlockIDI, _ := c.Vars.Get("newPropBlockID")
			newPropBlockID := newPropBlockIDI.(*ttypes.BlockID)
			newVote, err := util.ChangeVote(faultyReplica, tMsg, newPropBlockID)
			if err != nil {
				return []*types.Message{}, false
			}
			newMsgB, err := util.Marshal(newVote)
			if err != nil {
				return []*types.Message{}, false
			}
			return []*types.Message{c.NewMessage(tMsg.SchedulerMessage, newMsgB)}, true
		}
	}

	newVote, err := util.ChangeVoteToNil(faultyReplica, tMsg)
	if err != nil {
		return []*types.Message{}, false
	}
	newMsgB, err := util.Marshal(newVote)
	if err != nil {
		return []*types.Message{}, false
	}
	return []*types.Message{c.NewMessage(tMsg.SchedulerMessage, newMsgB)}, true
}

func (testCaseThreeFilters) round0(c *smlib.Context) ([]*types.Message, bool) {
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
	if tMsg.Type == util.Proposal {
		blockID, ok := util.GetProposalBlockIDS(tMsg)
		if ok {
			c.Vars.Set("oldProposal", blockID)
		}
		return []*types.Message{tMsg.SchedulerMessage}, true
	}

	partition := getReplicaPartition(c.Context)
	honestDelayed, _ := partition.GetPart("honestDelayed")

	if honestDelayed.Contains(tMsg.From) && tMsg.Type == util.Prevote {
		return []*types.Message{}, true
	}
	return []*types.Message{tMsg.SchedulerMessage}, true
}

func (testCaseThreeFilters) higherRound(c *smlib.Context) ([]*types.Message, bool) {
	tMsg, err := util.GetMessageFromSendEvent(c.CurEvent, c.MessagePool)
	if err != nil {
		return []*types.Message{}, false
	}
	_, round := util.ExtractHR(tMsg)
	if round == -1 {
		return []*types.Message{tMsg.SchedulerMessage}, true
	}
	if round == 0 {
		return []*types.Message{}, false
	}

	if tMsg.Type != util.Proposal {
		return []*types.Message{tMsg.SchedulerMessage}, true
	}

	curState := c.StateMachine.CurState()
	if curState.Is(stateForceRelock) {
		return []*types.Message{}, false
	}

	blockID, ok := util.GetProposalBlockID(tMsg)
	if !ok {
		return []*types.Message{}, false
	}
	blockIDS := blockID.Hash.String()
	oldProposal, _ := c.Vars.GetString("oldProposal")

	if blockIDS != oldProposal {
		c.Vars.Set("newPropBlockID", blockID)
		c.Vars.Set("newProposal", blockIDS)
		return []*types.Message{tMsg.SchedulerMessage}, true
	}

	return []*types.Message{}, true
}

type testCaseThreeCond struct{}

func (testCaseThreeCond) diffProposal(c *smlib.Context) bool {
	tMsg, err := util.GetMessageFromSendEvent(c.CurEvent, c.MessagePool)
	if err != nil {
		return false
	}

	if tMsg.Type != util.Proposal {
		return false
	}
	blockIDS, ok := util.GetProposalBlockIDS(tMsg)
	if !ok {
		return false
	}
	oldProposal, _ := c.Vars.GetString("oldProposal")
	if blockIDS != oldProposal {
		blockID, ok := util.GetProposalBlockID(tMsg)
		if ok {
			c.Vars.Set("newPropBlockID", blockID)
		}
		c.Vars.Set("newProposal", blockIDS)
		return true
	}
	return false
}

func (testCaseThreeCond) nextRound(c *smlib.Context) bool {
	tMsg, err := util.GetMessageFromSendEvent(c.CurEvent, c.MessagePool)
	if err != nil {
		return false
	}
	_, round := util.ExtractHR(tMsg)
	rI, ok := c.Vars.Get("nextRoundCount")
	if !ok {
		c.Vars.Set("nextRoundCount", map[string]int{})
		rI, _ = c.Vars.Get("nextRoundCount")
	}
	nextRoundCount := rI.(map[string]int)
	cRound, ok := nextRoundCount[string(tMsg.From)]
	if !ok || cRound < round {
		nextRoundCount[string(tMsg.From)] = round
	}
	c.Vars.Set("nextRoundCount", nextRoundCount)

	curRound, _ := c.Vars.GetInt("curRound")

	skipped := 0
	for _, r := range nextRoundCount {
		if r > curRound {
			skipped++
		}
	}
	if skipped == c.Replicas.Cap() {
		c.Logger().Info("Reached next round")
		return true
	}
	return false
}

func (testCaseThreeCond) oldVote(c *smlib.Context) bool {
	tMsg, err := util.GetMessageFromSendEvent(c.CurEvent, c.MessagePool)
	if err != nil {
		return false
	}
	if tMsg.Type != util.Precommit {
		return false
	}
	partition := getReplicaPartition(c.Context)
	honestDelayed, _ := partition.GetPart("honestDelayed")
	replica, _ := c.Replicas.Get(tMsg.From)

	if !honestDelayed.Contains(tMsg.From) || !util.IsVoteFrom(tMsg, replica) {
		return false
	}

	oldProposal, _ := c.Vars.GetString("oldProposal")

	blockID, ok := util.GetVoteBlockIDS(tMsg)
	if !ok {
		return false
	}
	c.Vars.Set("blockVote", blockID)
	c.Logger().With(log.LogParams{
		"cur_vote":     blockID,
		"old_proposal": oldProposal,
	}).Info("Checking vote == old proposal")
	return blockID == oldProposal
}

func (testCaseThreeCond) newVote(c *smlib.Context) bool {
	tMsg, err := util.GetMessageFromSendEvent(c.CurEvent, c.MessagePool)
	if err != nil {
		return false
	}
	if tMsg.Type != util.Precommit {
		return false
	}
	partition := getReplicaPartition(c.Context)
	honestDelayed, _ := partition.GetPart("honestDelayed")
	replica, _ := c.Replicas.Get(tMsg.From)

	if !honestDelayed.Contains(tMsg.From) || !util.IsVoteFrom(tMsg, replica) {
		return false
	}
	newProposal, _ := c.Vars.GetString("newProposal")

	blockID, ok := util.GetVoteBlockIDS(tMsg)
	if !ok {
		return false
	}
	c.Vars.Set("blockVote", blockID)
	c.Logger().With(log.LogParams{
		"cur_vote":     blockID,
		"new_proposal": newProposal,
	}).Info("Checking vote == new proposal")
	if blockID == newProposal {
		c.EndTestCase()
		return true
	}
	return false
}

func testCaseThreeSetup(c *testlib.Context) error {
	faults := int((c.Replicas.Cap() - 1) / 3)
	partition, _ := util.
		NewGenericParititioner(c.Replicas).
		CreateParition([]int{faults, 1, 2 * faults}, []string{"faulty", "honestDelayed", "rest"})
	c.Vars.Set("partition", partition)
	c.Vars.Set("faults", faults)
	c.Logger().With(log.LogParams{
		"partition": partition.String(),
	}).Info("Partitiion created")
	return nil
}

func Three() *testlib.TestCase {

	filters := testCaseThreeFilters{}
	cond := testCaseThreeCond{}
	commonConds := commonCond{}

	sm := smlib.NewStateMachine()
	relocked := sm.Builder().
		On(commonConds.valueLockedCond, stateLockedValue).
		On(commonConds.roundReached(1), stateRound1).
		On(cond.diffProposal, stateForceRelock)

	relocked.On(cond.oldVote, smlib.FailStateLabel)
	relocked.On(cond.newVote, smlib.SuccessStateLabel)

	handler := smlib.NewAsyncStateMachineHandler(sm)
	handler.AddEventHandler(filters.faultyVoteFilter)
	handler.AddEventHandler(filters.round0)
	handler.AddEventHandler(filters.higherRound)

	testcase := testlib.NewTestCase("ChangeLockedValue", 70*time.Second, handler)
	testcase.SetupFunc(testCaseThreeSetup)
	testcase.AssertFn(func(c *testlib.Context) bool {
		return sm.CurState().Is(smlib.SuccessStateLabel)
	})
	return testcase
}
