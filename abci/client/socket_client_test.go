package abcicli_test

import (
	"errors"
	"testing"
	"time"

	"github.com/tendermint/abci/client"
)

func TestSocketClientStopForErrorDeadlock(t *testing.T) {
	c := abcicli.NewSocketClient(":80", false)
	err := errors.New("foo-tendermint")

	// See Issue https://github.com/tendermint/abci/issues/114
	doneChan := make(chan bool)
	go func() {
		defer close(doneChan)
		c.StopForError(err)
		c.StopForError(err)
	}()

	select {
	case <-doneChan:
	case <-time.After(time.Second * 4):
		t.Fatalf("Test took too long, potential deadlock still exists")
	}
}
