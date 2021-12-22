package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/log"
)

type testService struct {
	BaseService
}

func (testService) OnStop() {}
func (testService) OnStart(context.Context) error {
	return nil
}

func TestBaseServiceWait(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ts := &testService{}
	ts.BaseService = *NewBaseService(log.TestingLogger(), "TestService", ts)
	err := ts.Start(ctx)
	require.NoError(t, err)

	waitFinished := make(chan struct{})
	go func() {
		ts.Wait()
		waitFinished <- struct{}{}
	}()

	go cancel()

	select {
	case <-waitFinished:
		// all good
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected Wait() to finish within 100 ms.")
	}
}
