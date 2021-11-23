package sanity

import (
	"time"

	"github.com/ds-test-framework/scheduler/log"
	"github.com/ds-test-framework/scheduler/testlib"
	smlib "github.com/ds-test-framework/scheduler/testlib/statemachine"
	"github.com/ds-test-framework/scheduler/types"
	"github.com/ds-test-framework/tendermint-test/util"
	ttypes "github.com/tendermint/tendermint/types"
)

type higherPropFilters struct{}

func (higherPropFilters) getFaultyReplica(c *smlib.Context) *types.Replica {
	fI, _ := c.Vars.Get("faultyReplica")
	return fI.(*types.Replica)
}

func (h higherPropFilters) faultyFilter(c *smlib.Context) ([]*types.Message, bool) {
	tMsg, err := util.GetMessageFromSendEvent(c.CurEvent, c.MessagePool)
	if err != nil {
		return []*types.Message{}, false
	}
	if tMsg.Type != util.Prevote && tMsg.Type != util.Precommit {
		return []*types.Message{}, false
	}

	// TODO: Change this to get the replica that the vote belongs to and check if its faulty.
	// Problem is faulty need not be just one replica
	faulty := h.getFaultyReplica(c)

	if !util.IsVoteFrom(tMsg, faulty) {
		return []*types.Message{}, false
	}

	partition := getPartition(c.Context)
	rest, _ := partition.GetPart("rest")
	replica, _ := c.Replicas.Get(tMsg.From)

	newProposal, ok := c.Vars.Get("newProposalBlockID")
	if ok && rest.Contains(tMsg.To) && tMsg.Type == util.Prevote {
		newVote, err := util.ChangeVote(replica, tMsg, newProposal.(*ttypes.BlockID))
		if err != nil {
			return []*types.Message{}, false
		}
		newMsgB, err := util.Marshal(newVote)
		if err != nil {
			return []*types.Message{}, false
		}
		return []*types.Message{c.NewMessage(tMsg.SchedulerMessage, newMsgB)}, true
	}

	newVote, err := util.ChangeVoteToNil(replica, tMsg)
	if err != nil {
		return []*types.Message{}, false
	}
	newMsgB, err := util.Marshal(newVote)
	if err != nil {
		return []*types.Message{}, false
	}
	return []*types.Message{c.NewMessage(tMsg.SchedulerMessage, newMsgB)}, true
}

func (higherPropFilters) round0(c *smlib.Context) ([]*types.Message, bool) {
	tMsg, err := util.GetMessageFromSendEvent(c.CurEvent, c.MessagePool)
	if err != nil {
		return []*types.Message{}, false
	}
	_, round := util.ExtractHR(tMsg)
	if round != 0 {
		return []*types.Message{}, false
	}
	if tMsg.Type == util.Proposal {
		blockIDS, ok := util.GetProposalBlockIDS(tMsg)
		if ok {
			c.Vars.Set("oldProposal", blockIDS)
		}
	}
	if tMsg.Type != util.Prevote {
		return []*types.Message{tMsg.SchedulerMessage}, true
	}

	partition := getPartition(c.Context)
	h, _ := partition.GetPart("honestDelayed")
	var hID types.ReplicaID
	for _, r := range h.ReplicaSet.Iter() {
		hID = r
	}
	hReplica, _ := c.Replicas.Get(hID)
	if util.IsVoteFrom(tMsg, hReplica) {
		return []*types.Message{}, true
	}
	return []*types.Message{tMsg.SchedulerMessage}, true
}

func (higherPropFilters) propFilter(c *smlib.Context) ([]*types.Message, bool) {
	tMsg, err := util.GetMessageFromSendEvent(c.CurEvent, c.MessagePool)
	if err != nil {
		return []*types.Message{}, false
	}
	if tMsg.Type != util.Proposal {
		return []*types.Message{}, false
	}
	_, round := util.ExtractHR(tMsg)
	if round == 0 {
		blockIDS, _ := util.GetProposalBlockIDS(tMsg)
		c.Vars.Set("oldProposal", blockIDS)
		return []*types.Message{tMsg.SchedulerMessage}, true
	}
	blockIDS, _ := util.GetProposalBlockIDS(tMsg)
	oldProp, _ := c.Vars.GetString("oldProposal")
	if blockIDS != oldProp {
		blockID, _ := util.GetProposalBlockID(tMsg)
		c.Vars.Set("newProposal", blockIDS)
		c.Vars.Set("newProposalBlockID", blockID)
		return []*types.Message{tMsg.SchedulerMessage}, true
	}
	return []*types.Message{}, true
}

type higherPropCond struct{}

func (higherPropCond) valueLockedCond(c *smlib.Context) bool {
	if !c.CurEvent.IsMessageReceive() {
		return false
	}
	messageID, _ := c.CurEvent.MessageID()
	message, ok := c.MessagePool.Get(messageID)
	if !ok {
		return false
	}

	tMsg, err := util.Unmarshal(message.Data)
	if err != nil {
		return false
	}

	partition := getPartition(c.Context)
	honestDelayed, _ := partition.GetPart("honestDelayed")

	if tMsg.Type == util.Prevote && honestDelayed.Contains(tMsg.To) {
		c.Logger().With(log.LogParams{
			"message_id": messageID,
		}).Debug("Prevote received by honest delayed")
		fI, _ := c.Vars.Get("faults")
		faults := fI.(int)

		voteBlockID, ok := util.GetVoteBlockIDS(tMsg)
		if ok {
			oldBlockID, ok := c.Vars.GetString("oldProposal")
			if ok && voteBlockID == oldBlockID {
				votes, ok := c.Vars.GetInt("prevotesSent")
				if !ok {
					votes = 0
				}
				votes++
				c.Vars.Set("prevotesSent", votes)
				if votes >= (2 * faults) {
					c.Logger().Info("2f+1 votes received! Value locked!")
					return true
				}
			}
		}
	}
	return false
}

func (higherPropCond) diffPropSeen(c *smlib.Context) bool {
	tMsg, err := util.GetMessageFromSendEvent(c.CurEvent, c.MessagePool)
	if err != nil {
		return false
	}
	if tMsg.Type != util.Proposal {
		return false
	}
	blockIDS, _ := util.GetProposalBlockIDS(tMsg)
	oldProp, ok := c.Vars.GetString("oldProposal")
	if !ok {
		return false
	}
	if blockIDS != oldProp {
		blockID, _ := util.GetProposalBlockID(tMsg)
		c.Vars.Set("newProposal", blockIDS)
		c.Vars.Set("newProposalBlockID", blockID)
		c.Logger().With(log.LogParams{
			"newProposal": blockIDS,
		}).Info("Setting new proposal")
		return true
	}
	return false
}

func (higherPropCond) newPropSeen(c *smlib.Context) bool {
	tMsg, err := util.GetMessageFromSendEvent(c.CurEvent, c.MessagePool)
	if err != nil {
		return false
	}
	if tMsg.Type != util.Proposal {
		return false
	}
	blockIDS, _ := util.GetProposalBlockIDS(tMsg)
	newProp, _ := c.Vars.GetString("newProposal")
	proposal := tMsg.Msg.GetProposal().Proposal
	if blockIDS == newProp && proposal.PolRound != -1 {
		c.Vars.Set("newPropReproposeRound", int(proposal.Round))
		c.Logger().With(log.LogParams{
			"round":       proposal.Round,
			"newProposal": newProp,
			"propBlockID": blockIDS,
			"polRound":    proposal.PolRound,
		}).Info("New proposal reproposed")
		return true
	}
	return false
}

func (higherPropCond) hNewVote(c *smlib.Context) bool {
	tMsg, err := util.GetMessageFromSendEvent(c.CurEvent, c.MessagePool)
	if err != nil {
		return false
	}
	partition := getPartition(c.Context)
	h, _ := partition.GetPart("honestDelayed")
	var hID types.ReplicaID
	for _, r := range h.ReplicaSet.Iter() {
		hID = r
	}
	hReplica, _ := c.Replicas.Get(hID)
	_, round := util.ExtractHR(tMsg)
	correctRound, _ := c.Vars.GetInt("newPropReproposeRound")
	if tMsg.Type != util.Prevote || !util.IsVoteFrom(tMsg, hReplica) {
		return false
	}
	newProp, _ := c.Vars.GetString("newProposal")
	blockIDS, _ := util.GetVoteBlockIDS(tMsg)
	return blockIDS == newProp && round == correctRound
}

func (higherPropCond) hOldVote(c *smlib.Context) bool {
	tMsg, err := util.GetMessageFromSendEvent(c.CurEvent, c.MessagePool)
	if err != nil {
		return false
	}
	partition := getPartition(c.Context)
	h, _ := partition.GetPart("honestDelayed")
	var hID types.ReplicaID
	for _, r := range h.ReplicaSet.Iter() {
		hID = r
	}
	hReplica, _ := c.Replicas.Get(hID)
	_, round := util.ExtractHR(tMsg)
	correctRound, _ := c.Vars.GetInt("newPropReproposeRound")
	if tMsg.Type != util.Prevote || !util.IsVoteFrom(tMsg, hReplica) {
		return false
	}
	oldProp, _ := c.Vars.GetString("oldProposal")
	blockIDS, _ := util.GetVoteBlockIDS(tMsg)
	return blockIDS == oldProp && round == correctRound
}

func (higherPropCond) rOldVote(c *smlib.Context) bool {
	tMsg, err := util.GetMessageFromSendEvent(c.CurEvent, c.MessagePool)
	if err != nil {
		return false
	}
	partition := getPartition(c.Context)
	rest, _ := partition.GetPart("rest")
	if tMsg.Type == util.Precommit && rest.Contains(tMsg.From) {
		oldProp, _ := c.Vars.GetString("oldProposal")
		voteBlockID, _ := util.GetVoteBlockIDS(tMsg)
		return oldProp == voteBlockID
	}
	return false
}

func higherPropSetup(c *testlib.Context) error {
	faults := int((c.Replicas.Cap() - 1) / 3)
	partition, _ := util.
		NewGenericParititioner(c.Replicas).
		CreateParition([]int{faults, 1, 2 * faults}, []string{"faulty", "honestDelayed", "rest"})

	faulty, _ := partition.GetPart("faulty")
	var faultyReplicaID types.ReplicaID
	for _, r := range faulty.ReplicaSet.Iter() {
		faultyReplicaID = r
	}
	faultyReplica, _ := c.Replicas.Get(faultyReplicaID)
	c.Vars.Set("faultyReplica", faultyReplica)
	c.Vars.Set("partition", partition)
	c.Vars.Set("faults", faults)
	c.Logger().With(log.LogParams{
		"partition": partition.String(),
	}).Info("Partitiion created")
	return nil
}

func HigherProp() *testlib.TestCase {
	filter := higherPropFilters{}
	cond := higherPropCond{}
	commonCond := commonCond{}

	sm := smlib.NewStateMachine()
	diffProposalSeen := sm.Builder().
		On(cond.valueLockedCond, "ValueLocked").
		On(commonCond.roundReached(1), "Round1").
		On(cond.diffPropSeen, "DiffProposal")
	diffProposalSeen.On(cond.rOldVote, smlib.FailStateLabel)
	newPropSeen := diffProposalSeen.On(cond.newPropSeen, "DiffProposalReproposed")
	newPropSeen.On(cond.hNewVote, smlib.SuccessStateLabel)
	newPropSeen.On(cond.hOldVote, smlib.FailStateLabel)

	handler := smlib.NewAsyncStateMachineHandler(sm)
	handler.AddEventHandler(filter.faultyFilter)
	handler.AddEventHandler(filter.round0)
	handler.AddEventHandler(filter.propFilter)

	testcase := testlib.NewTestCase("HigherLockedRoundProp", 3*time.Minute, handler)
	testcase.SetupFunc(higherPropSetup)
	testcase.AssertFn(func(c *testlib.Context) bool {
		return sm.CurState().Success
	})
	return testcase
}
