package sync_test

import (
	"testing"

	tmsync "github.com/tendermint/tendermint/libs/sync"
)

func TestCloser(t *testing.T) {
	closer := tmsync.NewCloser()

	for i := 0; i < 10; i++ {
		closer.Close()
	}

	<-closer.Done()
}
