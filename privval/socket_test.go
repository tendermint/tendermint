package privval

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"

	p2pconn "github.com/tendermint/tendermint/p2p/conn"
	"github.com/tendermint/tendermint/types"
)

func TestSocketPVAddress(t *testing.T) {
	var (
		chainID = cmn.RandStr(12)
		sc, rs  = testSetupSocketPair(t, chainID)
	)
	defer sc.Stop()
	defer rs.Stop()

	serverAddr := rs.privVal.GetAddress()

	clientAddr, err := sc.getAddress()
	require.NoError(t, err)

	assert.Equal(t, serverAddr, clientAddr)

	// TODO(xla): Remove when PrivValidator2 replaced PrivValidator.
	assert.Equal(t, serverAddr, sc.GetAddress())

}

func TestSocketPVPubKey(t *testing.T) {
	var (
		chainID = cmn.RandStr(12)
		sc, rs  = testSetupSocketPair(t, chainID)
	)
	defer sc.Stop()
	defer rs.Stop()

	clientKey, err := sc.getPubKey()
	require.NoError(t, err)

	privKey := rs.privVal.GetPubKey()

	assert.Equal(t, privKey, clientKey)

	// TODO(xla): Remove when PrivValidator2 replaced PrivValidator.
	assert.Equal(t, privKey, sc.GetPubKey())
}

func TestSocketPVProposal(t *testing.T) {
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

func TestSocketPVVote(t *testing.T) {
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

func TestSocketPVHeartbeat(t *testing.T) {
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

func TestSocketPVAcceptDeadline(t *testing.T) {
	var (
		sc = NewSocketPV(
			log.TestingLogger(),
			"127.0.0.1:0",
			crypto.GenPrivKeyEd25519(),
		)
	)
	defer sc.Stop()

	SocketPVAcceptDeadline(time.Millisecond)(sc)

	assert.Equal(t, sc.Start().(cmn.Error).Data(), ErrConnWaitTimeout)
}

func TestSocketPVDeadline(t *testing.T) {
	var (
		addr    = testFreeAddr(t)
		listenc = make(chan struct{})
		sc      = NewSocketPV(
			log.TestingLogger(),
			addr,
			crypto.GenPrivKeyEd25519(),
		)
	)

	SocketPVConnDeadline(100 * time.Millisecond)(sc)
	SocketPVConnWait(500 * time.Millisecond)(sc)

	go func(sc *SocketPV) {
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
			crypto.GenPrivKeyEd25519(),
		)
		if err == nil {
			break
		}
	}

	<-listenc

	// Sleep to guarantee deadline has been hit.
	time.Sleep(20 * time.Microsecond)

	_, err := sc.getPubKey()
	assert.Equal(t, err.(cmn.Error).Data(), ErrConnTimeout)
}

func TestSocketPVWait(t *testing.T) {
	sc := NewSocketPV(
		log.TestingLogger(),
		"127.0.0.1:0",
		crypto.GenPrivKeyEd25519(),
	)
	defer sc.Stop()

	SocketPVConnWait(time.Millisecond)(sc)

	assert.Equal(t, sc.Start().(cmn.Error).Data(), ErrConnWaitTimeout)
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
		types.NewMockPV(),
		crypto.GenPrivKeyEd25519(),
	)
	defer rs.Stop()

	RemoteSignerConnDeadline(time.Millisecond)(rs)
	RemoteSignerConnRetries(retries)(rs)

	assert.Equal(t, rs.Start().(cmn.Error).Data(), ErrDialRetryMax)

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
) (*SocketPV, *RemoteSigner) {
	var (
		addr    = testFreeAddr(t)
		logger  = log.TestingLogger()
		privVal = types.NewMockPV()
		readyc  = make(chan struct{})
		rs      = NewRemoteSigner(
			logger,
			chainID,
			addr,
			privVal,
			crypto.GenPrivKeyEd25519(),
		)
		sc = NewSocketPV(
			logger,
			addr,
			crypto.GenPrivKeyEd25519(),
		)
	)

	go func(sc *SocketPV) {
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
