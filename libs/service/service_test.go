package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/log"
)

type testService struct {
	started bool
	stopped bool
	mu      sync.Mutex
	BaseService
}

func (t *testService) OnStop() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stopped = true
}
func (t *testService) OnStart(context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.started = true
	return nil
}

func (t *testService) isStarted() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.started
}

func (t *testService) isStopped() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.stopped
}

func TestBaseService(t *testing.T) {
	t.Cleanup(leaktest.Check(t))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()

	t.Run("Wait", func(t *testing.T) {
		wctx, wcancel := context.WithCancel(ctx)
		defer wcacnel()
		ts := &testService{}
		ts.BaseService = *NewBaseService(logger, t.Name(), ts)
		err := ts.Start(wctx)
		require.NoError(t, err)
		require.True(t, ts.isStarted())

		waitFinished := make(chan struct{})
		go func() {
			ts.Wait()
			waitFinished <- struct{}{}
		}()

		go wcancel()

		select {
		case <-waitFinished:
			require.True(t, ts.isStopped())

		case <-time.After(100 * time.Millisecond):
			t.Fatal("expected Wait() to finish within 100 ms.")
		}
	})
	t.Run("ManualStop", func(t *testing.T) {
		ts := &testService{}
		ts.BaseService = *NewBaseService(logger, t.Name(), ts)

	})

}
