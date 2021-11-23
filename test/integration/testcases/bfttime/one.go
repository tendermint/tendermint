package bfttime

import (
	"strconv"
	"time"

	"github.com/ds-test-framework/scheduler/log"
	"github.com/ds-test-framework/scheduler/testlib"
	"github.com/ds-test-framework/scheduler/types"
	"github.com/ds-test-framework/tendermint-test/util"
)

func changeVoteFilter(c *testlib.Context) []*types.Message {
	if !c.CurEvent.IsMessageSend() {
		switch cEventType := c.CurEvent.Type.(type) {
		case *types.GenericEventType:
			if cEventType.T == "NewProposal" {
				height, ok := cEventType.Params["height"]
				if ok && height == "2" {
					timestampI, ok := cEventType.Params["block_timestamp"]
					if !ok {
						return []*types.Message{}
					}
					timestamp, err := strconv.ParseInt(timestampI, 10, 64)
					if err != nil {
						return []*types.Message{}
					}
					c.Logger().With(log.LogParams{
						"height":    height,
						"block_id":  cEventType.Params["blockID"],
						"timestamp": time.Unix(timestamp, 0),
					}).Info("Received new height proposal")
					c.Vars.Set("newtimestamp", time.Unix(timestamp, 0))
				}
			}
		}
		return []*types.Message{}
	}
	messageID, _ := c.CurEvent.MessageID()
	message, ok := c.MessagePool.Get(messageID)
	if !ok {
		return []*types.Message{}
	}

	tMsg, err := util.Unmarshal(message.Data)
	if err != nil {
		return []*types.Message{message}
	}

	if tMsg.Type != util.Precommit {
		return []*types.Message{message}
	}

	curTime, _ := util.GetVoteTime(tMsg)

	replica, ok := c.Replicas.Get(message.From)
	if !ok {
		c.Logger().With(log.LogParams{
			"replica": message.From,
		}).Warn("Could not fetch replica information")
		return []*types.Message{message}
	}
	newVote, err := util.ChangeVoteTime(replica, tMsg, curTime.Add(24*time.Hour))
	if err != nil {
		c.Logger().With(log.LogParams{
			"error": err,
		}).Warn("Could not change vote time")
		return []*types.Message{message}
	}
	newMsgB, err := util.Marshal(newVote)
	if err != nil {
		return []*types.Message{message}
	}
	c.Logger().With(log.LogParams{
		"from": message.From,
		"to":   message.To,
		"type": tMsg.Type,
	}).Info("Changed the vote time")
	return []*types.Message{c.NewMessage(message, newMsgB)}
}

func OneTestCase() *testlib.TestCase {

	testcase := testlib.NewTestCase("BFTTimeOne", 50*time.Second, testlib.NewGenericHandler(changeVoteFilter))
	testcase.AssertFn(func(c *testlib.Context) bool {
		newTimestampI, ok := c.Vars.Get("newtimestamp")
		if !ok {
			return false
		}
		newTimestamp := newTimestampI.(time.Time)
		c.Logger().With(log.LogParams{
			"newtimestamp": newTimestamp.String(),
			"current_time": time.Now().String(),
		}).Info("Checking timestamp of new block")
		return newTimestamp.After(time.Now().Add(23 * time.Hour))
	})

	return testcase
}
