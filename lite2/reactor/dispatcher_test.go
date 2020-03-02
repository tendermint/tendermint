package reactor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/p2p/mocks"
)

type testRequest struct {
	CallID uint64
	Value  string
}

func (m *testRequest) GetCallID() uint64    { return m.CallID }
func (m *testRequest) SetCallID(id uint64)  { m.CallID = id }
func (m *testRequest) ValidateBasic() error { return nil }

type testResponse struct {
	CallID uint64
	Value  string
}

func (m *testResponse) GetCallID() uint64    { return m.CallID }
func (m *testResponse) SetCallID(id uint64)  { m.CallID = id }
func (m *testResponse) ValidateBasic() error { return nil }

func TestDispatcher_call_respond(t *testing.T) {
	dispatcher := NewDispatcher()
	req := &testRequest{Value: "foo"}
	resp := &testResponse{Value: "bar"}

	peer := &mocks.Peer{}
	peer.On("ID").Return(p2p.ID("id"))
	peer.On("Send", LiteChannel, cdc.MustMarshalBinaryBare(req)).Run(func(args mock.Arguments) {
		err := dispatcher.respond(peer, resp)
		require.NoError(t, err)
	}).Return(true)

	r, err := dispatcher.call(peer, req)
	require.NoError(t, err)
	assert.Equal(t, r, resp)
}

func TestDispatcher_call_respondMultiple(t *testing.T) {
	dispatcher := NewDispatcher()
	req := &testRequest{Value: "foo"}
	resp := &testResponse{Value: "bar"}

	peer := &mocks.Peer{}
	peer.On("ID").Return(p2p.ID("id"))
	peer.On("Send", LiteChannel, cdc.MustMarshalBinaryBare(req)).Run(func(args mock.Arguments) {
		err := dispatcher.respond(peer, resp)
		require.NoError(t, err)
		err = dispatcher.respond(peer, resp)
		require.Error(t, err)
	}).Return(true)

	r, err := dispatcher.call(peer, req)
	require.NoError(t, err)
	assert.Equal(t, r, resp)
}

func TestDispatcher_call_timeout(t *testing.T) {
	dispatcher := NewDispatcher()
	dispatcher.timeout = 100 * time.Millisecond

	peer := &mocks.Peer{}
	peer.On("ID").Return(p2p.ID("id"))
	peer.On("Send", LiteChannel, mock.Anything).Run(func(args mock.Arguments) {
		go func() {
			time.Sleep(200 * time.Millisecond)
			err := dispatcher.respond(peer, &testResponse{})
			require.Error(t, err)
		}()
	}).Return(true)

	_, err := dispatcher.call(peer, &testRequest{})
	require.Error(t, err)
}

func TestDispatcher_call_sendFailure(t *testing.T) {
	dispatcher := NewDispatcher()

	peer := &mocks.Peer{}
	peer.On("ID").Return(p2p.ID("id"))
	peer.On("Send", LiteChannel, mock.Anything).Return(false)

	_, err := dispatcher.call(peer, &testRequest{})
	require.Error(t, err)
}

func TestDispatcher_call_invalidCallID(t *testing.T) {
	dispatcher := NewDispatcher()
	dispatcher.timeout = 100 * time.Millisecond

	peer := &mocks.Peer{}
	peer.On("ID").Return(p2p.ID("id"))
	peer.On("Send", LiteChannel, mock.Anything).Run(func(args mock.Arguments) {
		err := dispatcher.respond(peer, &testResponse{CallID: 99})
		require.Error(t, err)
	}).Return(true)

	_, err := dispatcher.call(peer, &testRequest{})
	require.Error(t, err)
}

func TestDispatcher_call_invalidPeerID(t *testing.T) {
	dispatcher := NewDispatcher()
	dispatcher.timeout = 100 * time.Millisecond

	peer := &mocks.Peer{}
	peer.On("ID").Return(p2p.ID("id"))
	peer.On("Send", LiteChannel, mock.Anything).Run(func(args mock.Arguments) {
		other := &mocks.Peer{}
		other.On("ID").Return(p2p.ID("other"))
		err := dispatcher.respond(other, &testResponse{})
		require.Error(t, err)
	}).Return(true)

	_, err := dispatcher.call(peer, &testRequest{})
	require.Error(t, err)
}
