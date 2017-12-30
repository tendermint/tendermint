package p2p

import (
	"context"
	"crypto/rand"
	"testing"

	crypto "github.com/libp2p/go-libp2p-crypto"
	lpeer "github.com/libp2p/go-libp2p-peer"
	ps "github.com/libp2p/go-libp2p-peerstore"
	swarm "github.com/libp2p/go-libp2p-swarm"
	bhost "github.com/libp2p/go-libp2p/p2p/host/basic"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/tendermint/tmlibs/log"
)

func TestListener(t *testing.T) {
	// Generate node PrivKey
	privKey, pubKey, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err.Error())
	}

	pid, err := lpeer.IDFromPublicKey(pubKey)
	if err != nil {
		t.Fatal(err.Error())
	}

	ps := ps.NewPeerstore()
	ps.AddPrivKey(pid, privKey)
	ps.AddPubKey(pid, pubKey)

	swarmNet, err := swarm.NewNetwork(context.Background(), nil, pid, ps, nil)
	if err != nil {
		t.Fatal(err.Error())
	}
	bh := bhost.New(swarmNet)

	// Create a listener
	maddr, err := ma.NewMultiaddr("/ip4/0.0.0.0/tcp/8001")
	if err != nil {
		t.Fatal(err.Error())
	}

	l, err := NewDefaultListener(maddr, true, log.TestingLogger(), swarmNet, bh)
	if err != nil {
		t.Fatal(err.Error())
	}
	_ = l
	l.Stop()

	// TODO: re-implement this test

	// Dial the listener
	/*
		lAddr := l.ExternalAddress()
		connOut, err := lAddr.Dial()
		if err != nil {
			t.Fatalf("Could not connect to listener address %v", lAddr)
		} else {
			t.Logf("Created a connection to listener address %v", lAddr)
		}
		connIn, ok := <-l.Connections()
		if !ok {
			t.Fatalf("Could not get inbound connection from listener")
		}

		msg := []byte("hi!")
		go func() {
			_, err := connIn.Write(msg)
			if err != nil {
				t.Error(err)
			}
		}()
		b := make([]byte, 32)
		n, err := connOut.Read(b)
		if err != nil {
			t.Fatalf("Error reading off connection: %v", err)
		}

		b = b[:n]
		if !bytes.Equal(msg, b) {
			t.Fatalf("Got %s, expected %s", b, msg)
		}

		// Close the server, no longer needed.
		l.Stop()
	*/
}
