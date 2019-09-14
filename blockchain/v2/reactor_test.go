package v2

import (
	"testing"
)

func TestReactor(t *testing.T) {
	reactor := NewReactor()
	reactor.Start()
	script := []Event{
		// TODO
	}

	for _, event := range script {
		reactor.Receive(event)
	}
	reactor.Stop()
}
