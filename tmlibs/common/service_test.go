package common

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type testService struct {
	BaseService
}

func (testService) OnReset() error {
	return nil
}

func TestBaseServiceWait(t *testing.T) {
	ts := &testService{}
	ts.BaseService = *NewBaseService(nil, "TestService", ts)
	ts.Start()

	waitFinished := make(chan struct{})
	go func() {
		ts.Wait()
		waitFinished <- struct{}{}
	}()

	go ts.Stop()

	select {
	case <-waitFinished:
		// all good
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected Wait() to finish within 100 ms.")
	}
}

func TestBaseServiceReset(t *testing.T) {
	ts := &testService{}
	ts.BaseService = *NewBaseService(nil, "TestService", ts)
	ts.Start()

	err := ts.Reset()
	require.Error(t, err, "expected cant reset service error")

	ts.Stop()

	err = ts.Reset()
	require.NoError(t, err)

	err = ts.Start()
	require.NoError(t, err)
}
