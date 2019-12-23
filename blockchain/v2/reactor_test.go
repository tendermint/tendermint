package v2

import (
	"testing"
)

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
		reactor.Receive(event)
	}
	reactor.Stop()
}
