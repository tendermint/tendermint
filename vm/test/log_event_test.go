package vm

import (
	. "github.com/tendermint/tendermint/vm"
	"testing"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/events"
	"github.com/tendermint/tendermint/types"
	"reflect"
)

var TOPICS_EXPECTED = []string{"0000000000000000000000000000000000000000000000000000000000000001", "0000000000000000000000000000000000000000000000000000000000000002", "0000000000000000000000000000000000000000000000000000000000000003", "0000000000000000000000000000000000000000000000000000000000000004"}
var DATA_EXPECTED = "10"
var HEIGHT_EXPECTED int64 = 0

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

	eventSwitch := &events.EventSwitch{}
	eventSwitch.Start()
	eventId := types.EventStringSolidityEvent(account2.Address.Bytes()[12:])
	
	doneChan := make(chan struct{}, 1)
	
	eventSwitch.AddListenerForEvent("test", eventId, func(evtIfc interface{}){
			event := evtIfc.(*SolLog)
			// No need to test address as this event would not happen if it wasn't correct
			if !reflect.DeepEqual(event.Topics, TOPICS_EXPECTED) {
				t.Errorf("Event topics are wrong. Got: %v. Expected: %v", event.Topics, TOPICS_EXPECTED)   
			}
			if event.Data != DATA_EXPECTED {
				t.Errorf("Event data is wrong. Got: %s. Expected: %s", event.Data, DATA_EXPECTED)
			}
			if event.Height != HEIGHT_EXPECTED {
				t.Errorf("Event block height is wrong. Got: %d. Expected: %d", event.Height, HEIGHT_EXPECTED)
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
		push1, 
		16, 	// data value
		push1, 
		0, 		// memory slot
		mstore8, 
		push1, 
		4, 		// topic 4
		push1, 
		3, 		// topic 3
		push1, 
		2, 		// topic 2
		push1, 
		1, 		// topic 1
		push1, 
		1, 		// size of data
		push1, 
		0, 		// data starts at this offset
		log4, 
		stop,
	}
	
	_, err := ourVm.Call(account1, account2, code, []byte{}, 0, &gas)
	<- doneChan
	if err != nil {
		t.Fatal(err)
	}
}
