package p2p

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
)

func TestRouter_ConstructQueueFactory(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("ValidateOptionsPopulatesDefaultQueue", func(t *testing.T) {
		opts := RouterOptions{}
		require.NoError(t, opts.Validate())
		require.Equal(t, "fifo", opts.QueueType)
	})
	t.Run("Default", func(t *testing.T) {
		require.Zero(t, os.Getenv("TM_P2P_QUEUE"))
		opts := RouterOptions{}
		r, err := NewRouter(log.NewNopLogger(), nil, nil, nil, func() *types.NodeInfo { return &types.NodeInfo{} }, nil, nil, opts)
		require.NoError(t, err)
		require.NoError(t, r.setupQueueFactory(ctx))

		_, ok := r.queueFactory(1).(*fifoQueue)
		require.True(t, ok)
	})
	t.Run("Fifo", func(t *testing.T) {
		opts := RouterOptions{QueueType: queueTypeFifo}
		r, err := NewRouter(log.NewNopLogger(), nil, nil, nil, func() *types.NodeInfo { return &types.NodeInfo{} }, nil, nil, opts)
		require.NoError(t, err)
		require.NoError(t, r.setupQueueFactory(ctx))

		_, ok := r.queueFactory(1).(*fifoQueue)
		require.True(t, ok)
	})
	t.Run("Priority", func(t *testing.T) {
		opts := RouterOptions{QueueType: queueTypePriority}
		r, err := NewRouter(log.NewNopLogger(), nil, nil, nil, func() *types.NodeInfo { return &types.NodeInfo{} }, nil, nil, opts)
		require.NoError(t, err)
		require.NoError(t, r.setupQueueFactory(ctx))

		q, ok := r.queueFactory(1).(*pqScheduler)
		require.True(t, ok)
		defer q.close()
	})
	t.Run("NonExistant", func(t *testing.T) {
		opts := RouterOptions{QueueType: "fast"}
		_, err := NewRouter(log.NewNopLogger(), nil, nil, nil, func() *types.NodeInfo { return &types.NodeInfo{} }, nil, nil, opts)
		require.Error(t, err)
		require.Contains(t, err.Error(), "fast")
	})
	t.Run("InternalsSafeWhenUnspecified", func(t *testing.T) {
		r := &Router{}
		require.Zero(t, r.options.QueueType)

		fn, err := r.createQueueFactory(ctx)
		require.Error(t, err)
		require.Nil(t, fn)
	})
}
