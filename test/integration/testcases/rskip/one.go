package rskip

import (
	"time"

	"github.com/ds-test-framework/scheduler/log"
	"github.com/ds-test-framework/scheduler/testlib"
	smlib "github.com/ds-test-framework/scheduler/testlib/statemachine"
	"github.com/ds-test-framework/scheduler/types"
	"github.com/ds-test-framework/tendermint-test/util"
)

func heightReached(height int) smlib.Condition {
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
		mHeight, _ := util.ExtractHR(tMsg)
		return mHeight >= height
	}
}

// type roundReachedCond struct {
// 	replicas map[types.ReplicaID]int
// 	lock     *sync.Mutex
// 	round    int
// }

// func newRoundReachedCond(round int) *roundReachedCond {
// 	return &roundReachedCond{
// 		replicas: make(map[types.ReplicaID]int),
// 		lock:     new(sync.Mutex),
// 		round:    round,
// 	}
// }

// func (r *roundReachedCond) Check(c *testlib.Context) bool {

// 	if !c.CurEvent.IsMessageSend() {
// 		return false
// 	}
// 	messageID, _ := c.CurEvent.MessageID()
// 	message, ok := c.MessagePool.Get(messageID)
// 	if !ok {
// 		return false
// 	}
// 	tMsg, err := util.Unmarshal(message.Data)
// 	if err != nil {
// 		return false
// 	}
// 	_, round := util.ExtractHR(tMsg)

// 	r.lock.Lock()
// 	r.replicas[message.From] = round
// 	r.lock.Unlock()

// 	threshold := int(c.Replicas.Cap() * 2 / 3)
// 	count := 0
// 	r.lock.Lock()
// 	for _, round := range r.replicas {
// 		if round >= r.round {
// 			count = count + 1
// 		}
// 	}
// 	r.lock.Unlock()
// 	return count >= threshold
// }

func roundReached(toRound int) smlib.Condition {
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
		if skipped == c.Replicas.Cap() {
			c.Logger().With(log.LogParams{"round": toRound}).Info("Reached round")
			c.Vars.Set("CurRound", toRound)
			return true
		}
		return false
	}
}

func setupFunc(c *testlib.Context) error {
	faults := int((c.Replicas.Cap() - 1) / 3)
	partitioner := util.NewStaticPartitioner(c.Replicas, faults)
	partitioner.NewPartition(0)
	partition, _ := partitioner.GetPartition(0)
	c.Vars.Set("partition", partition)
	delayedMessages := types.NewMessageStore()
	c.Vars.Set("delayedMessages", delayedMessages)
	return nil
}

func getPartition(c *testlib.Context) *util.Partition {
	v, _ := c.Vars.Get("partition")
	return v.(*util.Partition)
}

func getDelayedMStore(c *testlib.Context) *types.MessageStore {
	v, _ := c.Vars.Get("delayedMessages")
	return v.(*types.MessageStore)
}

func noDelayedMessagesCond(c *smlib.Context) bool {
	delayedMessages := getDelayedMStore(c.Context)
	return delayedMessages.Size() == 0
}

func changeVoteFilter(height, round int) smlib.EventHandler {
	return func(c *smlib.Context) ([]*types.Message, bool) {
		if !c.CurEvent.IsMessageSend() {
			return []*types.Message{}, true
		}
		messageID, _ := c.CurEvent.MessageID()
		message, ok := c.MessagePool.Get(messageID)
		if !ok {
			return []*types.Message{}, false
		}

		tMsg, err := util.Unmarshal(message.Data)
		if err != nil {
			return []*types.Message{}, false
		}
		if tMsg.Type != util.Prevote {
			return []*types.Message{message}, false
		}
		h, r := util.ExtractHR(tMsg)
		if h != height || r >= round || r < 0 {
			return []*types.Message{message}, true
		}

		partition := getPartition(c.Context)
		rest, _ := partition.GetPart("rest")
		faulty, _ := partition.GetPart("faulty")
		if rest.Contains(message.From) {
			return []*types.Message{message}, true
		} else if faulty.Contains(message.From) {
			replica, ok := c.Replicas.Get(message.From)
			if !ok {
				return []*types.Message{}, false
			}
			newvote, err := util.ChangeVoteToNil(replica, tMsg)
			if err != nil {
				return []*types.Message{}, false
			}
			data, err := util.Marshal(newvote)
			if err != nil {
				return []*types.Message{}, false
			}
			return []*types.Message{c.NewMessage(message, data)}, true
		} else {
			delayedM := getDelayedMStore(c.Context)
			delayedM.Add(message)
			return []*types.Message{}, true
		}
	}
}

func deliverDelayedFilter(c *smlib.Context) ([]*types.Message, bool) {
	if c.StateMachine.CurState().Label != "deliverDelayed" {
		return []*types.Message{}, false
	}
	if !c.CurEvent.IsMessageSend() {
		return []*types.Message{}, true
	}
	messageID, _ := c.CurEvent.MessageID()
	message, ok := c.MessagePool.Get(messageID)
	if !ok {
		return []*types.Message{}, false
	}
	delayedM := getDelayedMStore(c.Context)
	if delayedM.Size() == 0 {
		return []*types.Message{message}, true
	}
	messages := make([]*types.Message, delayedM.Size())
	for i, m := range delayedM.Iter() {
		messages[i] = m
		delayedM.Remove(m.ID)
	}
	messages = append(messages, message)
	return messages, true
}

func OneTestcase(height, round int) *testlib.TestCase {

	sm := smlib.NewStateMachine()
	sm.Builder().
		On(heightReached(height), "delayAndChangeVotes").
		On(roundReached(round), "deliverDelayed").
		On(noDelayedMessagesCond, smlib.SuccessStateLabel)

	handler := smlib.NewAsyncStateMachineHandler(sm)
	handler.AddEventHandler(changeVoteFilter(height, round))
	handler.AddEventHandler(deliverDelayedFilter)

	testcase := testlib.NewTestCase("RoundSkipPrevote", 30*time.Second, handler)
	testcase.SetupFunc(setupFunc)
	testcase.AssertFn(func(c *testlib.Context) bool {
		curRound, ok := c.Vars.GetInt("CurRound")
		return ok && curRound == round
	})

	return testcase
}
