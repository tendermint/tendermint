package p2ptest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/internal/p2p"
	"github.com/tendermint/tendermint/types"
)

// RequireEmpty requires that the given channel is empty.
func RequireEmpty(ctx context.Context, t *testing.T, channels ...*p2p.Channel) {
	t.Helper()

	ctx, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
	defer cancel()

	iter := p2p.MergedChannelIterator(ctx, channels...)
	count := 0
	for iter.Next(ctx) {
		count++
		require.Nil(t, iter.Envelope())
	}
	require.Zero(t, count)
	require.Error(t, ctx.Err())
}

// RequireReceive requires that the given envelope is received on the channel.
func RequireReceive(ctx context.Context, t *testing.T, channel *p2p.Channel, expect p2p.Envelope) {
	t.Helper()

	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	iter := channel.Receive(ctx)
	count := 0
	for iter.Next(ctx) {
		count++
		envelope := iter.Envelope()
		require.Equal(t, expect.From, envelope.From)
		require.Equal(t, expect.Message, envelope.Message)
	}

	if !assert.True(t, count >= 1) {
		require.NoError(t, ctx.Err(), "timed out waiting for message %v", expect)
	}
}

// RequireReceiveUnordered requires that the given envelopes are all received on
// the channel, ignoring order.
func RequireReceiveUnordered(ctx context.Context, t *testing.T, channel *p2p.Channel, expect []*p2p.Envelope) {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	actual := []*p2p.Envelope{}

	iter := channel.Receive(ctx)
	for iter.Next(ctx) {
		actual = append(actual, iter.Envelope())
		if len(actual) == len(expect) {
			require.ElementsMatch(t, expect, actual, "len=%d", len(actual))
			return
		}
	}

	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		require.ElementsMatch(t, expect, actual)
	}
}

// RequireSend requires that the given envelope is sent on the channel.
func RequireSend(ctx context.Context, t *testing.T, channel *p2p.Channel, envelope p2p.Envelope) {
	tctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	err := channel.Send(tctx, envelope)
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		require.Fail(t, "timed out sending message to %q", envelope.To)
	default:
		require.NoError(t, err, "unexpected error")
	}
}

// RequireSendReceive requires that a given Protobuf message is sent to the
// given peer, and then that the given response is received back.
func RequireSendReceive(
	ctx context.Context,
	t *testing.T,
	channel *p2p.Channel,
	peerID types.NodeID,
	send proto.Message,
	receive proto.Message,
) {
	RequireSend(ctx, t, channel, p2p.Envelope{To: peerID, Message: send})
	RequireReceive(ctx, t, channel, p2p.Envelope{From: peerID, Message: send})
}

// RequireNoUpdates requires that a PeerUpdates subscription is empty.
func RequireNoUpdates(ctx context.Context, t *testing.T, peerUpdates *p2p.PeerUpdates) {
	t.Helper()
	select {
	case update := <-peerUpdates.Updates():
		if ctx.Err() == nil {
			require.Fail(t, "unexpected peer updates", "got %v", update)
		}
	case <-ctx.Done():
	default:
	}
}

// RequireError requires that the given peer error is submitted for a peer.
func RequireError(ctx context.Context, t *testing.T, channel *p2p.Channel, peerError p2p.PeerError) {
	tctx, tcancel := context.WithTimeout(ctx, time.Second)
	defer tcancel()

	err := channel.SendError(tctx, peerError)
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		require.Fail(t, "timed out reporting error", "%v for %q", peerError, channel.String())
	default:
		require.NoError(t, err, "unexpected error")
	}
}

// RequireUpdate requires that a PeerUpdates subscription yields the given update.
func RequireUpdate(t *testing.T, peerUpdates *p2p.PeerUpdates, expect p2p.PeerUpdate) {
	timer := time.NewTimer(time.Second) // not time.After due to goroutine leaks
	defer timer.Stop()

	select {
	case update := <-peerUpdates.Updates():
		require.Equal(t, expect.NodeID, update.NodeID, "node id did not match")
		require.Equal(t, expect.Status, update.Status, "statuses did not match")
	case <-timer.C:
		require.Fail(t, "timed out waiting for peer update", "expected %v", expect)
	}
}

// RequireUpdates requires that a PeerUpdates subscription yields the given updates
// in the given order.
func RequireUpdates(t *testing.T, peerUpdates *p2p.PeerUpdates, expect []p2p.PeerUpdate) {
	timer := time.NewTimer(time.Second) // not time.After due to goroutine leaks
	defer timer.Stop()

	actual := []p2p.PeerUpdate{}
	for {
		select {
		case update := <-peerUpdates.Updates():
			actual = append(actual, update)
			if len(actual) == len(expect) {
				for idx := range expect {
					require.Equal(t, expect[idx].NodeID, actual[idx].NodeID)
					require.Equal(t, expect[idx].Status, actual[idx].Status)
				}

				return
			}

		case <-timer.C:
			require.Equal(t, expect, actual, "did not receive expected peer updates")
			return
		}
	}
}
