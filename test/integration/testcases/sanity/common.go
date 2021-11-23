package sanity

import (
	"github.com/ds-test-framework/scheduler/testlib"
	smlib "github.com/ds-test-framework/scheduler/testlib/statemachine"
	"github.com/ds-test-framework/scheduler/types"
	"github.com/ds-test-framework/tendermint-test/util"
)

type commonCond struct{}

func (commonCond) commitCond(c *smlib.Context) bool {
	cEventType := c.CurEvent.Type
	switch cEventType := cEventType.(type) {
	case *types.GenericEventType:
		if cEventType.T != "Committing block" {
			return false
		}
		blockID, ok := cEventType.Params["block_id"]
		if ok {
			c.Vars.Set("CommitBlockID", blockID)
		}
		c.Vars.Set("Committed", true)
		return true
	}
	return false
}

func (commonCond) roundReached(toRound int) smlib.Condition {
	return func(c *smlib.Context) bool {
		tMsg, err := util.GetMessageFromSendEvent(c.CurEvent, c.MessagePool)
		if err != nil {
			return false
		}
		_, round := util.ExtractHR(tMsg)
		rI, ok := c.Vars.Get("roundCount")
		if !ok {
			c.Vars.Set("roundCount", make(map[string]int))
			rI, _ = c.Vars.Get("roundCount")
		}
		roundCount := rI.(map[string]int)
		cRound, ok := roundCount[string(tMsg.From)]
		if !ok {
			roundCount[string(tMsg.From)] = round
		}
		if cRound < round {
			roundCount[string(tMsg.From)] = round
		}
		c.Vars.Set("roundCount", roundCount)

		skipped := 0
		for _, r := range roundCount {
			if r >= toRound {
				skipped++
			}
		}
		if skipped == c.Replicas.Cap() {
			c.Vars.Set("CurRound", toRound)
			return true
		}
		return false
	}
}

type commonFilters struct{}

func (commonFilters) isFaultyVote(c *testlib.Context, tMsg *util.TMessageWrapper) bool {
	partition := getPartition(c)
	faulty, _ := partition.GetPart("faulty")
	return (tMsg.Type == util.Prevote || tMsg.Type == util.Precommit) && faulty.Contains(tMsg.From)
}

func (commonFilters) changeFultyVote(c *testlib.Context, tMsg *util.TMessageWrapper) *types.Message {
	partition := getPartition(c)
	faulty, _ := partition.GetPart("faulty")
	if (tMsg.Type == util.Prevote || tMsg.Type == util.Precommit) && faulty.Contains(tMsg.From) {
		replica, _ := c.Replicas.Get(tMsg.From)
		newVote, err := util.ChangeVoteToNil(replica, tMsg)
		if err != nil {
			return tMsg.SchedulerMessage
		}
		newMsgB, err := util.Marshal(newVote)
		if err != nil {
			return tMsg.SchedulerMessage
		}
		return c.NewMessage(tMsg.SchedulerMessage, newMsgB)
	}
	return tMsg.SchedulerMessage
}

func (t commonFilters) faultyFilter(c *smlib.Context) ([]*types.Message, bool) {
	tMsg, err := util.GetMessageFromSendEvent(c.CurEvent, c.MessagePool)
	if err != nil {
		return []*types.Message{}, false
	}
	if t.isFaultyVote(c.Context, tMsg) {
		return []*types.Message{t.changeFultyVote(c.Context, tMsg)}, true
	}
	return []*types.Message{}, false
}
