package v2

import "testing"

// XXX: This makes assumptions about the message routing
func TestReactor(t *testing.T) {
	reactor := Reactor{}
	reactor.Start()
	script := []Event{
		struct{}{},
	}

	for _, event := range script {
		reactor.Receive(event)
	}
	reactor.Wait()
}
