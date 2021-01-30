package p2p_test

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
)

func TestMemoryTransport(t *testing.T) {
	ctx := context.Background()
	network := p2p.NewMemoryNetwork(log.TestingLogger())
	a := network.CreateTransport("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	b := network.CreateTransport("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	c := network.CreateTransport("cccccccccccccccccccccccccccccccccccccccc")

	// Dialing a missing endpoint should fail.
	_, err := a.Dial(ctx, p2p.Endpoint{
		Protocol: p2p.MemoryProtocol,
		Path:     "dddddddddddddddddddddddddddddddddddddddd",
	})
	require.Error(t, err)

	// Dialing and accepting a→b and a→c should work.
	aToB, bToA, err := a.DialAccept(ctx, b)
	require.NoError(t, err)
	defer aToB.Close()
	defer bToA.Close()

	aToC, cToA, err := a.DialAccept(ctx, c)
	require.NoError(t, err)
	defer aToC.Close()
	defer cToA.Close()

	// Send and receive a message both ways a→b and b→a
	sent, err := aToB.SendMessage(1, []byte{0x01})
	require.NoError(t, err)
	require.True(t, sent)

	ch, msg, err := bToA.ReceiveMessage()
	require.NoError(t, err)
	require.EqualValues(t, 1, ch)
	require.EqualValues(t, []byte{0x01}, msg)

	sent, err = bToA.SendMessage(1, []byte{0x02})
	require.NoError(t, err)
	require.True(t, sent)

	ch, msg, err = aToB.ReceiveMessage()
	require.NoError(t, err)
	require.EqualValues(t, 1, ch)
	require.EqualValues(t, []byte{0x02}, msg)

	// Send and receive a message both ways a→c and c→a
	sent, err = aToC.SendMessage(1, []byte{0x03})
	require.NoError(t, err)
	require.True(t, sent)

	ch, msg, err = cToA.ReceiveMessage()
	require.NoError(t, err)
	require.EqualValues(t, 1, ch)
	require.EqualValues(t, []byte{0x03}, msg)

	sent, err = cToA.SendMessage(1, []byte{0x04})
	require.NoError(t, err)
	require.True(t, sent)

	ch, msg, err = aToC.ReceiveMessage()
	require.NoError(t, err)
	require.EqualValues(t, 1, ch)
	require.EqualValues(t, []byte{0x04}, msg)

	// If we close aToB, sending and receiving on either end will error.
	err = aToB.Close()
	require.NoError(t, err)

	_, err = aToB.SendMessage(1, []byte{0x05})
	require.Equal(t, io.EOF, err)

	_, _, err = aToB.ReceiveMessage()
	require.Equal(t, io.EOF, err)

	_, err = bToA.SendMessage(1, []byte{0x06})
	require.Equal(t, io.EOF, err)

	_, _, err = bToA.ReceiveMessage()
	require.Equal(t, io.EOF, err)

	// We can still send aToC.
	sent, err = aToC.SendMessage(1, []byte{0x07})
	require.NoError(t, err)
	require.True(t, sent)

	ch, msg, err = cToA.ReceiveMessage()
	require.NoError(t, err)
	require.EqualValues(t, 1, ch)
	require.EqualValues(t, []byte{0x07}, msg)

	// If we close the c transport, it will no longer accept connections,
	// but we can still use the open connection.
	endpoint := c.Endpoints()[0]
	err = c.Close()
	require.NoError(t, err)
	require.Empty(t, c.Endpoints())

	_, err = a.Dial(ctx, endpoint)
	require.Error(t, err)

	sent, err = aToC.SendMessage(1, []byte{0x08})
	require.NoError(t, err)
	require.True(t, sent)

	ch, msg, err = cToA.ReceiveMessage()
	require.NoError(t, err)
	require.EqualValues(t, 1, ch)
	require.EqualValues(t, []byte{0x08}, msg)
}
