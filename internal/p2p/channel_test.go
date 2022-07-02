package p2p

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/stretchr/testify/require"
)

type channelInternal struct {
	In    chan Envelope
	Out   chan Envelope
	Error chan PeerError
}

func testChannel(size int) (*channelInternal, *Channel) {
	in := &channelInternal{
		In:    make(chan Envelope, size),
		Out:   make(chan Envelope, size),
		Error: make(chan PeerError, size),
	}
	ch := &Channel{
		inCh:  in.In,
		outCh: in.Out,
		errCh: in.Error,
	}
	return in, ch
}

func TestChannel(t *testing.T) {
	t.Cleanup(leaktest.Check(t))

	bctx, bcancel := context.WithCancel(context.Background())
	defer bcancel()

	testCases := []struct {
		Name string
		Case func(context.Context, *testing.T)
	}{
		{
			Name: "Send",
			Case: func(ctx context.Context, t *testing.T) {
				ins, ch := testChannel(1)
				require.NoError(t, ch.Send(ctx, Envelope{From: "kip", To: "merlin"}))

				res, ok := <-ins.Out
				require.True(t, ok)
				require.EqualValues(t, "kip", res.From)
				require.EqualValues(t, "merlin", res.To)
			},
		},
		{
			Name: "SendError",
			Case: func(ctx context.Context, t *testing.T) {
				ins, ch := testChannel(1)
				require.NoError(t, ch.SendError(ctx, PeerError{NodeID: "kip", Err: errors.New("merlin")}))

				res, ok := <-ins.Error
				require.True(t, ok)
				require.EqualValues(t, "kip", res.NodeID)
				require.EqualValues(t, "merlin", res.Err.Error())
			},
		},
		{
			Name: "SendWithCanceledContext",
			Case: func(ctx context.Context, t *testing.T) {
				_, ch := testChannel(0)
				cctx, ccancel := context.WithCancel(ctx)
				ccancel()
				require.Error(t, ch.Send(cctx, Envelope{From: "kip", To: "merlin"}))
			},
		},
		{
			Name: "SendErrorWithCanceledContext",
			Case: func(ctx context.Context, t *testing.T) {
				_, ch := testChannel(0)
				cctx, ccancel := context.WithCancel(ctx)
				ccancel()

				require.Error(t, ch.SendError(cctx, PeerError{NodeID: "kip", Err: errors.New("merlin")}))
			},
		},
		{
			Name: "ReceiveEmptyIteratorBlocks",
			Case: func(ctx context.Context, t *testing.T) {
				_, ch := testChannel(1)
				iter := ch.Receive(ctx)
				require.NotNil(t, iter)
				out := make(chan bool)
				go func() {
					defer close(out)
					select {
					case <-ctx.Done():
					case out <- iter.Next(ctx):
					}
				}()
				select {
				case <-time.After(10 * time.Millisecond):
				case <-out:
					require.Fail(t, "iterator should not advance")
				}
				require.Nil(t, iter.Envelope())
			},
		},
		{
			Name: "ReceiveWithData",
			Case: func(ctx context.Context, t *testing.T) {
				ins, ch := testChannel(1)
				ins.In <- Envelope{From: "kip", To: "merlin"}
				iter := ch.Receive(ctx)
				require.NotNil(t, iter)
				require.True(t, iter.Next(ctx))

				res := iter.Envelope()
				require.EqualValues(t, "kip", res.From)
				require.EqualValues(t, "merlin", res.To)
			},
		},
		{
			Name: "ReceiveWithCanceledContext",
			Case: func(ctx context.Context, t *testing.T) {
				_, ch := testChannel(0)
				cctx, ccancel := context.WithCancel(ctx)
				ccancel()

				iter := ch.Receive(cctx)
				require.NotNil(t, iter)
				require.False(t, iter.Next(cctx))
				require.Nil(t, iter.Envelope())
			},
		},
		{
			Name: "IteratorWithCanceledContext",
			Case: func(ctx context.Context, t *testing.T) {
				_, ch := testChannel(0)

				iter := ch.Receive(ctx)
				require.NotNil(t, iter)

				cctx, ccancel := context.WithCancel(ctx)
				ccancel()
				require.False(t, iter.Next(cctx))
				require.Nil(t, iter.Envelope())
			},
		},
		{
			Name: "IteratorCanceledAfterFirstUseBecomesNil",
			Case: func(ctx context.Context, t *testing.T) {
				ins, ch := testChannel(1)

				ins.In <- Envelope{From: "kip", To: "merlin"}
				iter := ch.Receive(ctx)
				require.NotNil(t, iter)

				require.True(t, iter.Next(ctx))

				res := iter.Envelope()
				require.EqualValues(t, "kip", res.From)
				require.EqualValues(t, "merlin", res.To)

				cctx, ccancel := context.WithCancel(ctx)
				ccancel()

				require.False(t, iter.Next(cctx))
				require.Nil(t, iter.Envelope())
			},
		},
		{
			Name: "IteratorMultipleNextCalls",
			Case: func(ctx context.Context, t *testing.T) {
				ins, ch := testChannel(1)

				ins.In <- Envelope{From: "kip", To: "merlin"}
				iter := ch.Receive(ctx)
				require.NotNil(t, iter)

				require.True(t, iter.Next(ctx))

				res := iter.Envelope()
				require.EqualValues(t, "kip", res.From)
				require.EqualValues(t, "merlin", res.To)

				res1 := iter.Envelope()
				require.Equal(t, res, res1)
			},
		},
		{
			Name: "IteratorProducesNilObjectBeforeNext",
			Case: func(ctx context.Context, t *testing.T) {
				ins, ch := testChannel(1)

				iter := ch.Receive(ctx)
				require.NotNil(t, iter)
				require.Nil(t, iter.Envelope())

				ins.In <- Envelope{From: "kip", To: "merlin"}
				require.NotNil(t, iter)
				require.True(t, iter.Next(ctx))

				res := iter.Envelope()
				require.NotNil(t, res)
				require.EqualValues(t, "kip", res.From)
				require.EqualValues(t, "merlin", res.To)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			t.Cleanup(leaktest.Check(t))

			ctx, cancel := context.WithCancel(bctx)
			defer cancel()

			tc.Case(ctx, t)
		})
	}
}
