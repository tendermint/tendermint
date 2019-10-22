package v2

import (
	"fmt"
	"testing"
)

/*
# What do we test in v1?

* TestFastSyncNoBlockResponse
	* test that we switch to consensus after not receiving a block for a certain amount of time
* TestFastSyncBadBlockStopsPeer
	* test that a bad block will evict the peer
* TestBcBlockRequestMessageValidateBasic
	* test the behaviour of bcBlockRequestMessage.ValidateBasic()
* TestBcBlockResponseMessageValidateBasic
	* test the validation of of the message body

# What do we want to test:
	* Initialization from disk store
	* BlockResponse from state
		* Invalid Request
		* Block Missing
		* Block Found
	* StatusRequest
		* Invalid Message
		* Valid Status
	* Termination
		* Timeout
		* Completion on sync blocks
*/

func TestReactor(t *testing.T) {
	var (
		bufferSize = 10
		reactor    = NewReactor(bufferSize)
	)

	reactor.Start()
	script := []Event{
		// TODO
	}

	for _, event := range script {
		fmt.Println(event)
		//TODO reactor.Receive(event)
	}
	reactor.Stop()
}
