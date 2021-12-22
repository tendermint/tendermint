package proxy

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	abciclient "github.com/tendermint/tendermint/abci/client"
	abcimocks "github.com/tendermint/tendermint/abci/client/mocks"
	"github.com/tendermint/tendermint/libs/log"
)

type noopStoppableClientImpl struct {
	abciclient.Client
	count int
}

func (c *noopStoppableClientImpl) Stop() error { c.count++; return nil }

func TestAppConns_Start_Stop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clientMock := &abcimocks.Client{}
	clientMock.On("Start", mock.Anything).Return(nil).Times(4)
	clientMock.On("Error").Return(nil)
	clientMock.On("Wait").Return(nil).Times(4)
	cl := &noopStoppableClientImpl{Client: clientMock}

	creatorCallCount := 0
	creator := func(logger log.Logger) (abciclient.Client, error) {
		creatorCallCount++
		return cl, nil
	}

	appConns := NewAppConns(creator, log.TestingLogger(), NopMetrics())

	err := appConns.Start(ctx)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	cancel()
	appConns.Wait()

	clientMock.AssertExpectations(t)
	assert.Equal(t, 4, cl.count)
	assert.Equal(t, 4, creatorCallCount)
}

// Upon failure, we call tmos.Kill
func TestAppConns_Failure(t *testing.T) {
	ok := make(chan struct{})
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGTERM)
	go func() {
		for range c {
			close(ok)
			return
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clientMock := &abcimocks.Client{}
	clientMock.On("SetLogger", mock.Anything).Return()
	clientMock.On("Start", mock.Anything).Return(nil)

	clientMock.On("Wait").Return(nil)
	clientMock.On("Error").Return(errors.New("EOF"))
	cl := &noopStoppableClientImpl{Client: clientMock}

	creator := func(log.Logger) (abciclient.Client, error) {
		return cl, nil
	}

	appConns := NewAppConns(creator, log.TestingLogger(), NopMetrics())

	err := appConns.Start(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { cancel(); appConns.Wait() })

	select {
	case <-ok:
		t.Log("SIGTERM successfully received")
	case <-time.After(5 * time.Second):
		t.Fatal("expected process to receive SIGTERM signal")
	}
}
