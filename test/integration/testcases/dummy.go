package testcases

import (
	"time"

	"github.com/ds-test-framework/scheduler/log"
	"github.com/ds-test-framework/scheduler/testlib"
	smlib "github.com/ds-test-framework/scheduler/testlib/statemachine"
	"github.com/ds-test-framework/scheduler/types"
	"github.com/ds-test-framework/tendermint-test/util"
)

func action(c *testlib.Context) []*types.Message {
	if !c.CurEvent.IsMessageSend() {
		return []*types.Message{}
	}
	messageID, _ := c.CurEvent.MessageID()
	message, ok := c.MessagePool.Get(messageID)
	if ok {
		return []*types.Message{message}
	}
	return []*types.Message{}
}

func actionSM(c *smlib.Context) ([]*types.Message, bool) {
	if !c.CurEvent.IsMessageSend() {
		return []*types.Message{}, false
	}
	messageID, _ := c.CurEvent.MessageID()
	message, ok := c.MessagePool.Get(messageID)
	if ok {
		return []*types.Message{message}, true
	}
	return []*types.Message{}, true
}

func cond(c *smlib.Context) bool {
	e := c.CurEvent
	switch e.Type.(type) {
	case *types.MessageSendEventType:
		eventType := e.Type.(*types.MessageSendEventType)
		c.Logger().With(log.LogParams{"message_id": eventType.MessageID}).Debug("Received message")
		messageRaw, ok := c.MessagePool.Get(eventType.MessageID)
		if ok {
			message, err := util.Unmarshal(messageRaw.Data)
			if err != nil {
				return false
			}
			if message.Type == util.Precommit {
				return true
			}
		}

	}
	return false
}

func DummyTestCase() *testlib.TestCase {
	testcase := testlib.NewTestCase("Dummy", 20*time.Second, testlib.NewGenericHandler(action))
	testcase.AssertFn(func(c *testlib.Context) bool {
		return true
	})
	return testcase
}

func DummyTestCaseStateMachine() *testlib.TestCase {
	sm := smlib.NewStateMachine()
	sm.Builder().On(cond, smlib.SuccessStateLabel)

	handler := smlib.NewAsyncStateMachineHandler(sm)
	handler.AddEventHandler(actionSM)

	testcase := testlib.NewTestCase("DummySM", 30*time.Second, handler)
	return testcase
}

// type dummyCond struct{}

// func (*dummyCond) Check(_ *testing.EventWrapper, _ *testing.VarSet) bool {
// 	return true
// }

// func NewDummtTestCase() *testing.TestCase {
// 	t := testing.NewTestCase("Dummy", 5*time.Second)

// 	t.StartState().Upon(&dummyCond{}, t.SuccessState())
// 	return t
// }
