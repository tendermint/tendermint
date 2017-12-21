package p2p

import (
	"crypto/rand"
	"errors"
	// golog "log"
	// "net"
	"testing"
	// "time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	crypto "github.com/libp2p/go-libp2p-crypto"
	// inet "github.com/libp2p/go-libp2p-net"
	lpeer "github.com/libp2p/go-libp2p-peer"
)

func TestPeerBasic(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	// simulate remote peer
	privKey, pubKey, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err.Error())
	}

	peerId, err := lpeer.IDFromPrivateKey(privKey)
	if err != nil {
		t.Fatal(err.Error())
	}

	/*
		rp := &remotePeer{PrivKey: privKey, Config: DefaultPeerConfig()}
		rp.Start()
		defer rp.Stop()
	*/

	p, err := createOutboundPeerAndPerformHandshake(peerId, DefaultPeerConfig())
	require.Nil(err)

	err = p.Start()
	require.Nil(err)
	defer p.Stop()

	assert.True(p.IsRunning())
	assert.True(p.IsOutbound())
	assert.False(p.IsPersistent())
	p.makePersistent()
	assert.True(p.IsPersistent())
	assert.Equal(peerId.String(), p.PeerID().String())
	assert.True(pubKey.Equals(p.PubKey()))
}

func TestPeerWithoutAuthEnc(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	config := DefaultPeerConfig()

	// simulate remote peer
	privKey, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err.Error())
	}
	/*
		rp := &remotePeer{PrivKey: privKey, Config: config}
		rp.Start()
		defer rp.Stop()
	*/

	peerId, err := lpeer.IDFromPrivateKey(privKey)
	if err != nil {
		t.Fatal(err.Error())
	}

	p, err := createOutboundPeerAndPerformHandshake(peerId, config)
	require.Nil(err)

	err = p.Start()
	require.Nil(err)
	defer p.Stop()

	assert.True(p.IsRunning())
}

func TestPeerSend(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	config := DefaultPeerConfig()

	// simulate remote peer
	privKey, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err.Error())
	}
	/*
		rp := &remotePeer{PrivKey: privKey, Config: config}
		rp.Start()
		defer rp.Stop()
	*/

	peerId, err := lpeer.IDFromPrivateKey(privKey)
	if err != nil {
		t.Fatal(err.Error())
	}

	p, err := createOutboundPeerAndPerformHandshake(peerId, config)
	require.Nil(err)

	err = p.Start()
	require.Nil(err)

	defer p.Stop()

	assert.True(p.CanSend(0x01))
	assert.True(p.Send(0x01, "Asylum"))
}

func createOutboundPeerAndPerformHandshake(id lpeer.ID, config *PeerConfig) (*peer, error) {
	_, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return nil, err
	}
	// TODO: determine how to build stream here.
	return nil, errors.New("not implemented")
	/*
		var stream inet.Stream
		p, err := newOutboundPeer(stream, reactorsByCh, chDescs, func(p Peer, r interface{}) {}, pk, config)
		if err != nil {
			return nil, err
		}
		err = p.HandshakeTimeout(&NodeInfo{
			PubKey:  pk.PubKey().Unwrap().(crypto.PubKeyEd25519),
			Moniker: "host_peer",
			Network: "testing",
			Version: "123.123.123",
		}, 1*time.Second)
		if err != nil {
			return nil, err
		}
		return p, nil
	*/
}
