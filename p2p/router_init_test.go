package p2p

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/log"
)

func safeSetEnvVar(t *testing.T, key, value string) func() {
	t.Helper()

	previous := os.Getenv(key)
	require.NoError(t, os.Setenv(key, value))
	require.Equal(t, value, os.Getenv(key))
	return func() {
		require.NoError(t, os.Setenv(key, previous))
		require.Equal(t, previous, os.Getenv(key))
	}
}

func TestRouter_ConstructQueueFactory(t *testing.T) {
	t.Run("ValidateOptionsPopulatesDefaultQueue", func(t *testing.T) {
		opts := RouterOptions{}
		require.NoError(t, opts.Validate())
		require.Equal(t, "fifo", opts.QueueType)
	})
	t.Run("EnvironmentConfiguration", func(t *testing.T) {
		require.Zero(t, os.Getenv("TM_P2P_QUEUE"))

		t.Run("DefaultFifo", func(t *testing.T) {
			opts := RouterOptions{}
			r, err := NewRouter(log.NewNopLogger(), nil, NodeInfo{}, nil, nil, nil, opts)
			require.NoError(t, err)
			_, ok := r.queueFactory(1).(*fifoQueue)
			require.True(t, ok)
		})

		t.Run("ExplictFifo", func(t *testing.T) {
			defer safeSetEnvVar(t, "TM_P2P_QUEUE", "fifo")()

			opts := RouterOptions{}
			r, err := NewRouter(log.NewNopLogger(), nil, NodeInfo{}, nil, nil, nil, opts)
			require.NoError(t, err)
			_, ok := r.queueFactory(1).(*fifoQueue)
			require.True(t, ok)
		})

		t.Run("Priority", func(t *testing.T) {
			defer safeSetEnvVar(t, "TM_P2P_QUEUE", "priority")()

			opts := RouterOptions{}
			r, err := NewRouter(log.NewNopLogger(), nil, NodeInfo{}, nil, nil, nil, opts)
			require.NoError(t, err)
			q, ok := r.queueFactory(1).(*pqScheduler)
			require.True(t, ok)
			defer q.close()
		})

		t.Run("WDRR", func(t *testing.T) {
			defer safeSetEnvVar(t, "TM_P2P_QUEUE", "wdrr")()

			opts := RouterOptions{}
			r, err := NewRouter(log.NewNopLogger(), nil, NodeInfo{}, nil, nil, nil, opts)
			require.NoError(t, err)
			q := r.queueFactory(1)
			_, ok := q.(*wdrrScheduler)
			require.True(t, ok, "%T", q)

			defer q.close()
		})
	})
	t.Run("ExplicitConfiguration", func(t *testing.T) {
		t.Run("Default", func(t *testing.T) {
			require.Zero(t, os.Getenv("TM_P2P_QUEUE"))
			opts := RouterOptions{}
			r, err := NewRouter(log.NewNopLogger(), nil, NodeInfo{}, nil, nil, nil, opts)
			require.NoError(t, err)
			_, ok := r.queueFactory(1).(*fifoQueue)
			require.True(t, ok)
		})
		t.Run("Fifo", func(t *testing.T) {
			opts := RouterOptions{QueueType: "fifo"}
			r, err := NewRouter(log.NewNopLogger(), nil, NodeInfo{}, nil, nil, nil, opts)
			require.NoError(t, err)
			_, ok := r.queueFactory(1).(*fifoQueue)
			require.True(t, ok)
		})
		t.Run("Priority", func(t *testing.T) {
			opts := RouterOptions{QueueType: "priority"}
			r, err := NewRouter(log.NewNopLogger(), nil, NodeInfo{}, nil, nil, nil, opts)
			require.NoError(t, err)
			q, ok := r.queueFactory(1).(*pqScheduler)
			require.True(t, ok)
			defer q.close()
		})
		t.Run("WDRR", func(t *testing.T) {
			opts := RouterOptions{QueueType: "wdrr"}
			r, err := NewRouter(log.NewNopLogger(), nil, NodeInfo{}, nil, nil, nil, opts)
			require.NoError(t, err)
			q, ok := r.queueFactory(1).(*wdrrScheduler)
			require.True(t, ok)
			defer q.close()
		})
	})
	t.Run("InvalidQueueConfiguration", func(t *testing.T) {
		t.Run("NonExistant", func(t *testing.T) {
			opts := RouterOptions{QueueType: "fast"}
			_, err := NewRouter(log.NewNopLogger(), nil, NodeInfo{}, nil, nil, nil, opts)
			require.Error(t, err)
			require.Contains(t, err.Error(), "fast")
		})
		t.Run("InternalsSafe", func(t *testing.T) {
			r := &Router{}
			require.Zero(t, r.options.QueueType)

			fn, err := r.createQueueFactory()
			require.Error(t, err)
			require.Nil(t, fn)
		})
	})
}
