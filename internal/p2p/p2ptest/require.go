package p2ptest

import (
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/internal/p2p"
	"github.com/tendermint/tendermint/types"
)

// RequireEmpty requires that the given channel is empty.
func RequireEmpty(t *testing.T, channels ...*p2p.Channel) {
	for _, channel := range channels {
		select {
		case e := <-channel.In:
			require.Fail(t, "unexpected message", "channel %v should be empty, got %v", channel.ID, e)
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// RequireReceive requires that the given envelope is received on the channel.
func RequireReceive(t *testing.T, channel *p2p.Channel, expect p2p.Envelope) {
	timer := time.NewTimer(time.Second) // not time.After due to goroutine leaks
	defer timer.Stop()

	select {
	case e, ok := <-channel.In:
		require.True(t, ok, "channel %v is closed", channel.ID)
		require.Equal(t, expect, e)

	case <-channel.Done():
		require.Fail(t, "channel %v is closed", channel.ID)

	case <-timer.C:
		require.Fail(t, "timed out waiting for message", "%v on channel %v", expect, channel.ID)
	}
}

// RequireReceiveUnordered requires that the given envelopes are all received on
// the channel, ignoring order.
func RequireReceiveUnordered(t *testing.T, channel *p2p.Channel, expect []p2p.Envelope) {
	timer := time.NewTimer(time.Second) // not time.After due to goroutine leaks
	defer timer.Stop()

	actual := []p2p.Envelope{}
	for {
		select {
		case e, ok := <-channel.In:
			require.True(t, ok, "channel %v is closed", channel.ID)
			actual = append(actual, e)
			if len(actual) == len(expect) {
				require.ElementsMatch(t, expect, actual)
				return
			}

		case <-channel.Done():
			require.Fail(t, "channel %v is closed", channel.ID)

		case <-timer.C:
			require.ElementsMatch(t, expect, actual)
			return
		}
	}

}

// RequireSend requires that the given envelope is sent on the channel.
func RequireSend(t *testing.T, channel *p2p.Channel, envelope p2p.Envelope) {
	timer := time.NewTimer(time.Second) // not time.After due to goroutine leaks
	defer timer.Stop()
	select {
	case channel.Out <- envelope:
	case <-timer.C:
		require.Fail(t, "timed out sending message", "%v on channel %v", envelope, channel.ID)
	}
}

// RequireSendReceive requires that a given Protobuf message is sent to the
// given peer, and then that the given response is received back.
func RequireSendReceive(
	t *testing.T,
	channel *p2p.Channel,
	peerID types.NodeID,
	send proto.Message,
	receive proto.Message,
) {
	RequireSend(t, channel, p2p.Envelope{To: peerID, Message: send})
	RequireReceive(t, channel, p2p.Envelope{From: peerID, Message: send})
}

// RequireNoUpdates requires that a PeerUpdates subscription is empty.
func RequireNoUpdates(t *testing.T, peerUpdates *p2p.PeerUpdates) {
	t.Helper()
	select {
	case update := <-peerUpdates.Updates():
		require.Fail(t, "unexpected peer updates", "got %v", update)
	default:
	}
}

// RequireError requires that the given peer error is submitted for a peer.
func RequireError(t *testing.T, channel *p2p.Channel, peerError p2p.PeerError) {
	timer := time.NewTimer(time.Second) // not time.After due to goroutine leaks
	defer timer.Stop()
	select {
	case channel.Error <- peerError:
	case <-timer.C:
		require.Fail(t, "timed out reporting error", "%v on %v", peerError, channel.ID)
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
	case <-peerUpdates.Done():
		require.Fail(t, "peer updates subscription is closed")
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

		case <-peerUpdates.Done():
			require.Fail(t, "peer updates subscription is closed")

		case <-timer.C:
			require.Equal(t, expect, actual, "did not receive expected peer updates")
			return
		}
	}
}
