package byzantine

import (
	"time"

	"github.com/ds-test-framework/scheduler/testlib"
	smlib "github.com/ds-test-framework/scheduler/testlib/statemachine"
	"github.com/ds-test-framework/scheduler/types"
	"github.com/ds-test-framework/tendermint-test/util"
)

func getRoundCond(toRound int) smlib.Condition {
	return func(c *smlib.Context) bool {
		if !c.CurEvent.IsMessageSend() {
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
		_, round := util.ExtractHR(tMsg)
		rI, ok := c.Vars.Get("roundCount")
		if !ok {
			c.Vars.Set("roundCount", map[string]int{})
			rI, _ = c.Vars.Get("roundCount")
		}
		roundCount := rI.(map[string]int)
		cRound, ok := roundCount[string(message.From)]
		if !ok {
			roundCount[string(message.From)] = round
		}
		if cRound < round {
			roundCount[string(message.From)] = round
		}
		c.Vars.Set("roundCount", roundCount)

		skipped := 0
		for _, r := range roundCount {
			if r >= toRound {
				skipped++
			}
		}
		return skipped == c.Replicas.Cap()
	}
}

func changeProposal(c *smlib.Context) ([]*types.Message, bool) {
	return []*types.Message{}, false
}

func changeVote(c *smlib.Context) ([]*types.Message, bool) {
	return []*types.Message{}, false
}

func changeBlockParts(c *smlib.Context) ([]*types.Message, bool) {
	return []*types.Message{}, false
}

func One() *testlib.TestCase {
	stateMachine := smlib.NewStateMachine()

	h := smlib.NewAsyncStateMachineHandler(stateMachine)
	h.AddEventHandler(changeProposal)
	h.AddEventHandler(changeVote)
	h.AddEventHandler(changeBlockParts)

	testcase := testlib.NewTestCase(
		"CompetingProposals",
		30*time.Second,
		h,
	)
	return testcase
}
