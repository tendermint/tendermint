package types

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	crypto "github.com/tendermint/go-crypto"
	cmn "github.com/tendermint/tmlibs/common"
	"github.com/tendermint/tmlibs/log"

	p2pconn "github.com/tendermint/tendermint/p2p/conn"
	"github.com/tendermint/tendermint/types"
)

func TestSocketClientAddress(t *testing.T) {
	var (
		chainID = cmn.RandStr(12)
		sc, rs  = testSetupSocketPair(t, chainID)
	)
	defer sc.Stop()
	defer rs.Stop()

	serverAddr, err := rs.privVal.Address()
	require.NoError(t, err)

	clientAddr, err := sc.Address()
	require.NoError(t, err)

	assert.Equal(t, serverAddr, clientAddr)

	// TODO(xla): Remove when PrivValidator2 replaced PrivValidator.
	assert.Equal(t, serverAddr, sc.GetAddress())

}

func TestSocketClientPubKey(t *testing.T) {
	var (
		chainID = cmn.RandStr(12)
		sc, rs  = testSetupSocketPair(t, chainID)
	)
	defer sc.Stop()
	defer rs.Stop()

	clientKey, err := sc.PubKey()
	require.NoError(t, err)

	privKey, err := rs.privVal.PubKey()
	require.NoError(t, err)

	assert.Equal(t, privKey, clientKey)

	// TODO(xla): Remove when PrivValidator2 replaced PrivValidator.
	assert.Equal(t, privKey, sc.GetPubKey())
}

func TestSocketClientProposal(t *testing.T) {
	var (
		chainID = cmn.RandStr(12)
		sc, rs  = testSetupSocketPair(t, chainID)

		ts             = time.Now()
		privProposal   = &types.Proposal{Timestamp: ts}
		clientProposal = &types.Proposal{Timestamp: ts}
	)
	defer sc.Stop()
	defer rs.Stop()

	require.NoError(t, rs.privVal.SignProposal(chainID, privProposal))
	require.NoError(t, sc.SignProposal(chainID, clientProposal))
	assert.Equal(t, privProposal.Signature, clientProposal.Signature)
}

func TestSocketClientVote(t *testing.T) {
	var (
		chainID = cmn.RandStr(12)
		sc, rs  = testSetupSocketPair(t, chainID)

		ts    = time.Now()
		vType = types.VoteTypePrecommit
		want  = &types.Vote{Timestamp: ts, Type: vType}
		have  = &types.Vote{Timestamp: ts, Type: vType}
	)
	defer sc.Stop()
	defer rs.Stop()

	require.NoError(t, rs.privVal.SignVote(chainID, want))
	require.NoError(t, sc.SignVote(chainID, have))
	assert.Equal(t, want.Signature, have.Signature)
}

func TestSocketClientHeartbeat(t *testing.T) {
	var (
		chainID = cmn.RandStr(12)
		sc, rs  = testSetupSocketPair(t, chainID)

		want = &types.Heartbeat{}
		have = &types.Heartbeat{}
	)
	defer sc.Stop()
	defer rs.Stop()

	require.NoError(t, rs.privVal.SignHeartbeat(chainID, want))
	require.NoError(t, sc.SignHeartbeat(chainID, have))
	assert.Equal(t, want.Signature, have.Signature)
}

func TestSocketClientAcceptDeadline(t *testing.T) {
	var (
		sc = NewSocketClient(
			log.TestingLogger(),
			"127.0.0.1:0",
			crypto.GenPrivKeyEd25519(),
		)
	)
	defer sc.Stop()

	SocketClientAcceptDeadline(time.Millisecond)(sc)

	assert.Equal(t, errors.Cause(sc.Start()), ErrConnWaitTimeout)
}

func TestSocketClientDeadline(t *testing.T) {
	var (
		addr    = testFreeAddr(t)
		listenc = make(chan struct{})
		sc      = NewSocketClient(
			log.TestingLogger(),
			addr,
			crypto.GenPrivKeyEd25519(),
		)
	)

	SocketClientConnDeadline(10 * time.Millisecond)(sc)
	SocketClientConnWait(500 * time.Millisecond)(sc)

	go func(sc *SocketClient) {
		defer close(listenc)

		require.NoError(t, sc.Start())

		assert.True(t, sc.IsRunning())
	}(sc)

	for {
		conn, err := cmn.Connect(addr)
		if err != nil {
			continue
		}

		_, err = p2pconn.MakeSecretConnection(
			conn,
			crypto.GenPrivKeyEd25519().Wrap(),
		)
		if err == nil {
			break
		}
	}

	<-listenc

	// Sleep to guarantee deadline has been hit.
	time.Sleep(20 * time.Microsecond)

	_, err := sc.PubKey()
	assert.Equal(t, errors.Cause(err), ErrConnTimeout)
}

func TestSocketClientWait(t *testing.T) {
	sc := NewSocketClient(
		log.TestingLogger(),
		"127.0.0.1:0",
		crypto.GenPrivKeyEd25519(),
	)
	defer sc.Stop()

	SocketClientConnWait(time.Millisecond)(sc)

	assert.Equal(t, errors.Cause(sc.Start()), ErrConnWaitTimeout)
}

func TestRemoteSignerRetry(t *testing.T) {
	var (
		attemptc = make(chan int)
		retries  = 2
	)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	go func(ln net.Listener, attemptc chan<- int) {
		attempts := 0

		for {
			conn, err := ln.Accept()
			require.NoError(t, err)

			err = conn.Close()
			require.NoError(t, err)

			attempts++

			if attempts == retries {
				attemptc <- attempts
				break
			}
		}
	}(ln, attemptc)

	rs := NewRemoteSigner(
		log.TestingLogger(),
		cmn.RandStr(12),
		ln.Addr().String(),
		NewTestPrivValidator(types.GenSigner()),
		crypto.GenPrivKeyEd25519(),
	)
	defer rs.Stop()

	RemoteSignerConnDeadline(time.Millisecond)(rs)
	RemoteSignerConnRetries(retries)(rs)

	assert.Equal(t, errors.Cause(rs.Start()), ErrDialRetryMax)

	select {
	case attempts := <-attemptc:
		assert.Equal(t, retries, attempts)
	case <-time.After(100 * time.Millisecond):
		t.Error("expected remote to observe connection attempts")
	}
}

func testSetupSocketPair(
	t *testing.T,
	chainID string,
) (*SocketClient, *RemoteSigner) {
	var (
		addr    = testFreeAddr(t)
		logger  = log.TestingLogger()
		signer  = types.GenSigner()
		privVal = NewTestPrivValidator(signer)
		readyc  = make(chan struct{})
		rs      = NewRemoteSigner(
			logger,
			chainID,
			addr,
			privVal,
			crypto.GenPrivKeyEd25519(),
		)
		sc = NewSocketClient(
			logger,
			addr,
			crypto.GenPrivKeyEd25519(),
		)
	)

	go func(sc *SocketClient) {
		require.NoError(t, sc.Start())
		assert.True(t, sc.IsRunning())

		readyc <- struct{}{}
	}(sc)

	RemoteSignerConnDeadline(time.Millisecond)(rs)
	RemoteSignerConnRetries(1e6)(rs)

	require.NoError(t, rs.Start())
	assert.True(t, rs.IsRunning())

	<-readyc

	return sc, rs
}

// testFreeAddr claims a free port so we don't block on listener being ready.
func testFreeAddr(t *testing.T) string {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	return fmt.Sprintf("127.0.0.1:%d", ln.Addr().(*net.TCPAddr).Port)
}
