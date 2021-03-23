package context

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestContext(t *testing.T) {
	bctx, bcancel := context.WithCancel(context.Background())
	defer bcancel()

	t.Run("TriggeredOnClose", func(t *testing.T) {
		ctx, cancel := context.WithCancel(bctx)
		defer cancel()
		var isCalled bool

		ch := make(chan struct{})
		_, _ = WithSignalChanelClose(ctx, ch, func() { isCalled = true })
		require.False(t, isCalled)
		close(ch)
		time.Sleep(time.Millisecond)
		require.True(t, isCalled)
	})
	t.Run("TriggeredOnCancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(bctx)
		defer cancel()
		var isCalled bool

		ch := make(chan struct{})
		_, _ = WithSignalChanelClose(ctx, ch, func() { isCalled = true })
		require.False(t, isCalled)
		cancel()
		time.Sleep(time.Millisecond)
		require.True(t, isCalled)
	})
	t.Run("CancelsContext", func(t *testing.T) {
		ch := make(chan struct{})
		ctx, _ := WithSignalChannel(bctx, ch)
		require.NoError(t, ctx.Err())
		close(ch)
		time.Sleep(time.Millisecond)
		require.Error(t, ctx.Err())

	})
	t.Run("TODOCancelsContext", func(t *testing.T) {
		ch := make(chan struct{})
		ctx, _ := TODO(ch)
		require.NoError(t, ctx.Err())
		close(ch)
		time.Sleep(time.Millisecond)
		require.Error(t, ctx.Err())
	})
}
