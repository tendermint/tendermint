package lockedvalue

import (
	"github.com/ds-test-framework/scheduler/log"
	"github.com/ds-test-framework/scheduler/testlib"
	smlib "github.com/ds-test-framework/scheduler/testlib/statemachine"
	"github.com/ds-test-framework/scheduler/types"
	"github.com/ds-test-framework/tendermint-test/util"
)

type commonCond struct{}

func (commonCond) updateRoundCount(c *smlib.Context, round int, from types.ReplicaID) map[string]int {
	rI, ok := c.Vars.Get("roundCount")
	if !ok {
		c.Vars.Set("roundCount", map[string]int{})
		rI, _ = c.Vars.Get("roundCount")
	}
	roundCount := rI.(map[string]int)
	cRound, ok := roundCount[string(from)]
	if !ok {
		roundCount[string(from)] = round
	}
	if cRound < round {
		roundCount[string(from)] = round
	}
	c.Vars.Set("roundCount", roundCount)
	return roundCount
}

func (t commonCond) roundReached(toRound int) smlib.Condition {
	return func(c *smlib.Context) bool {
		tMsg, err := util.GetMessageFromSendEvent(c.CurEvent, c.MessagePool)
		if err != nil {
			return false
		}
		_, round := util.ExtractHR(tMsg)
		roundCount := t.updateRoundCount(c, round, tMsg.From)

		skipped := 0
		for _, r := range roundCount {
			if r >= toRound {
				skipped++
			}
		}
		if skipped == c.Replicas.Cap() {
			c.Logger().With(log.LogParams{"round": toRound}).Info("Reached round")
			return true
		}
		return false
	}
}

func (commonCond) valueLockedCond(c *smlib.Context) bool {
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

	partition := getReplicaPartition(c.Context)
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

type commonUtil struct{}

func (commonUtil) isFaultyVote(c *testlib.Context, tMsg *util.TMessageWrapper) bool {
	partition := getReplicaPartition(c)
	faulty, _ := partition.GetPart("faulty")
	return (tMsg.Type == util.Prevote || tMsg.Type == util.Precommit) && faulty.Contains(tMsg.From)
}

func (commonUtil) changeFultyVote(c *testlib.Context, tMsg *util.TMessageWrapper) *types.Message {
	partition := getReplicaPartition(c)
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
