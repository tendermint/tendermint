package proxy

import (
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
)

func TestAppConns_Start_Stop(t *testing.T) {
	quitCh := make(<-chan struct{})

	clientMock := &abcimocks.Client{}
	clientMock.On("SetLogger", mock.Anything).Return().Times(4)
	clientMock.On("Start").Return(nil).Times(4)
	clientMock.On("Stop").Return(nil).Times(4)
	clientMock.On("Quit").Return(quitCh).Times(4)

	creatorCallCount := 0
	creator := func() (abciclient.Client, error) {
		creatorCallCount++
		return clientMock, nil
	}

	appConns := NewAppConns(creator)

	err := appConns.Start()
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	err = appConns.Stop()
	require.NoError(t, err)

	clientMock.AssertExpectations(t)
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
		}
	}()

	quitCh := make(chan struct{})
	var recvQuitCh <-chan struct{} // nolint:gosimple
	recvQuitCh = quitCh

	clientMock := &abcimocks.Client{}
	clientMock.On("SetLogger", mock.Anything).Return()
	clientMock.On("Start").Return(nil)
	clientMock.On("Stop").Return(nil)

	clientMock.On("Quit").Return(recvQuitCh)
	clientMock.On("Error").Return(errors.New("EOF")).Once()

	creator := func() (abciclient.Client, error) {
		return clientMock, nil
	}

	appConns := NewAppConns(creator)

	err := appConns.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := appConns.Stop(); err != nil {
			t.Error(err)
		}
	})

	// simulate failure
	close(quitCh)

	select {
	case <-ok:
		t.Log("SIGTERM successfully received")
	case <-time.After(5 * time.Second):
		t.Fatal("expected process to receive SIGTERM signal")
	}
}
