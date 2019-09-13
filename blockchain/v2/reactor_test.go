package v2

import (
	"testing"

	"github.com/tendermint/tendermint/libs/log"
)

func TestReactor(t *testing.T) {
	reactor := NewReactor()
	reactor.Start()
	reactor.setLogger(log.TestingLogger())
	script := []Event{
		// TODO
	}

	for _, event := range script {
		reactor.Receive(event)
	}
	reactor.Stop()
}
