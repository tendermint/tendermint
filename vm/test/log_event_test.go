package vm

import (
	"bytes"
	"reflect"
	"testing"

	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/events"
	"github.com/tendermint/tendermint/types"
	. "github.com/tendermint/tendermint/vm"
)

var expectedData = []byte{0x10}
var expectedHeight int64 = 0
var expectedTopics = []Word256{
	Int64ToWord256(1),
	Int64ToWord256(2),
	Int64ToWord256(3),
	Int64ToWord256(4)}

// Tests logs and events.
func TestLog4(t *testing.T) {

	st := newAppState()
	// Create accounts
	account1 := &Account{
		Address: LeftPadWord256(makeBytes(20)),
	}
	account2 := &Account{
		Address: LeftPadWord256(makeBytes(20)),
	}
	st.accounts[account1.Address.String()] = account1
	st.accounts[account2.Address.String()] = account2

	ourVm := NewVM(st, newParams(), Zero256, nil)

	eventSwitch := events.NewEventSwitch()
	_, err := eventSwitch.Start()
	if err != nil {
		t.Errorf("Failed to start eventSwitch: %v", err)
	}
	eventId := types.EventStringLogEvent(account2.Address.Postfix(20))

	doneChan := make(chan struct{}, 1)

	eventSwitch.AddListenerForEvent("test", eventId, func(event types.EventData) {
		logEvent := event.(types.EventDataLog)
		// No need to test address as this event would not happen if it wasn't correct
		if !reflect.DeepEqual(logEvent.Topics, expectedTopics) {
			t.Errorf("Event topics are wrong. Got: %v. Expected: %v", logEvent.Topics, expectedTopics)
		}
		if !bytes.Equal(logEvent.Data, expectedData) {
			t.Errorf("Event data is wrong. Got: %s. Expected: %s", logEvent.Data, expectedData)
		}
		if logEvent.Height != expectedHeight {
			t.Errorf("Event block height is wrong. Got: %d. Expected: %d", logEvent.Height, expectedHeight)
		}
		doneChan <- struct{}{}
	})

	ourVm.SetFireable(eventSwitch)

	var gas int64 = 100000

	mstore8 := byte(MSTORE8)
	push1 := byte(PUSH1)
	log4 := byte(LOG4)
	stop := byte(STOP)

	code := []byte{
		push1, 16, // data value
		push1, 0, // memory slot
		mstore8,
		push1, 4, // topic 4
		push1, 3, // topic 3
		push1, 2, // topic 2
		push1, 1, // topic 1
		push1, 1, // size of data
		push1, 0, // data starts at this offset
		log4,
		stop,
	}

	_, err = ourVm.Call(account1, account2, code, []byte{}, 0, &gas)
	<-doneChan
	if err != nil {
		t.Fatal(err)
	}
}
