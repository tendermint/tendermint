package privval

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto/ed25519"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"

	p2pconn "github.com/tendermint/tendermint/p2p/conn"
	"github.com/tendermint/tendermint/types"
)

func TestSocketPVAddress(t *testing.T) {
	var (
		chainID = cmn.RandStr(12)
		sc, rs  = testSetupSocketPair(t, chainID, types.NewMockPV())
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
		sc, rs  = testSetupSocketPair(t, chainID, types.NewMockPV())
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
		sc, rs  = testSetupSocketPair(t, chainID, types.NewMockPV())

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
		sc, rs  = testSetupSocketPair(t, chainID, types.NewMockPV())

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
		sc, rs  = testSetupSocketPair(t, chainID, types.NewMockPV())

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
			ed25519.GenPrivKey(),
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
			ed25519.GenPrivKey(),
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
			ed25519.GenPrivKey(),
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
		ed25519.GenPrivKey(),
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
		ed25519.GenPrivKey(),
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

func TestRemoteSignVoteErrors(t *testing.T) {
	var (
		chainID = cmn.RandStr(12)
		sc, rs  = testSetupSocketPair(t, chainID, types.NewErroringMockPV())

		ts    = time.Now()
		vType = types.VoteTypePrecommit
		vote  = &types.Vote{Timestamp: ts, Type: vType}
	)
	defer sc.Stop()
	defer rs.Stop()

	err := writeMsg(sc.conn, &SignVoteRequest{Vote: vote})
	require.NoError(t, err)

	res, err := readMsg(sc.conn)
	require.NoError(t, err)

	resp := *res.(*SignedVoteResponse)
	require.NotNil(t, resp.Error)
	require.Equal(t, resp.Error.Description, types.ErroringMockPVErr.Error())

	err = rs.privVal.SignVote(chainID, vote)
	require.Error(t, err)
	err = sc.SignVote(chainID, vote)
	require.Error(t, err)
}

func TestRemoteSignProposalErrors(t *testing.T) {
	var (
		chainID = cmn.RandStr(12)
		sc, rs  = testSetupSocketPair(t, chainID, types.NewErroringMockPV())

		ts       = time.Now()
		proposal = &types.Proposal{Timestamp: ts}
	)
	defer sc.Stop()
	defer rs.Stop()

	err := writeMsg(sc.conn, &SignProposalRequest{Proposal: proposal})
	require.NoError(t, err)

	res, err := readMsg(sc.conn)
	require.NoError(t, err)

	resp := *res.(*SignedProposalResponse)
	require.NotNil(t, resp.Error)
	require.Equal(t, resp.Error.Description, types.ErroringMockPVErr.Error())

	err = rs.privVal.SignProposal(chainID, proposal)
	require.Error(t, err)

	err = sc.SignProposal(chainID, proposal)
	require.Error(t, err)
}

func TestRemoteSignHeartbeatErrors(t *testing.T) {
	var (
		chainID = cmn.RandStr(12)
		sc, rs  = testSetupSocketPair(t, chainID, types.NewErroringMockPV())
		hb      = &types.Heartbeat{}
	)
	defer sc.Stop()
	defer rs.Stop()

	err := writeMsg(sc.conn, &SignHeartbeatRequest{Heartbeat: hb})
	require.NoError(t, err)

	res, err := readMsg(sc.conn)
	require.NoError(t, err)

	resp := *res.(*SignedHeartbeatResponse)
	require.NotNil(t, resp.Error)
	require.Equal(t, resp.Error.Description, types.ErroringMockPVErr.Error())

	err = rs.privVal.SignHeartbeat(chainID, hb)
	require.Error(t, err)

	err = sc.SignHeartbeat(chainID, hb)
	require.Error(t, err)
}

func TestErrUnexpectedResponse(t *testing.T) {
	var (
		addr    = testFreeAddr(t)
		logger  = log.TestingLogger()
		chainID = cmn.RandStr(12)
		readyc  = make(chan struct{})
		errc    = make(chan error, 1)

		rs = NewRemoteSigner(
			logger,
			chainID,
			addr,
			types.NewMockPV(),
			ed25519.GenPrivKey(),
		)
		sc = NewSocketPV(
			logger,
			addr,
			ed25519.GenPrivKey(),
		)
	)

	testStartSocketPV(t, readyc, sc)
	defer sc.Stop()
	RemoteSignerConnDeadline(time.Millisecond)(rs)
	RemoteSignerConnRetries(1e6)(rs)

	// we do not want to Start() the remote signer here and instead use the connection to
	// reply with intentionally wrong replies below:
	rsConn, err := rs.connect()
	defer rsConn.Close()
	require.NoError(t, err)
	require.NotNil(t, rsConn)
	<-readyc

	// Heartbeat:
	go func(errc chan error) {
		errc <- sc.SignHeartbeat(chainID, &types.Heartbeat{})
	}(errc)
	// read request and write wrong response:
	go testReadWriteResponse(t, &SignedVoteResponse{}, rsConn)
	err = <-errc
	require.Error(t, err)
	require.Equal(t, err, ErrUnexpectedResponse)

	// Proposal:
	go func(errc chan error) {
		errc <- sc.SignProposal(chainID, &types.Proposal{})
	}(errc)
	// read request and write wrong response:
	go testReadWriteResponse(t, &SignedHeartbeatResponse{}, rsConn)
	err = <-errc
	require.Error(t, err)
	require.Equal(t, err, ErrUnexpectedResponse)

	// Vote:
	go func(errc chan error) {
		errc <- sc.SignVote(chainID, &types.Vote{})
	}(errc)
	// read request and write wrong response:
	go testReadWriteResponse(t, &SignedHeartbeatResponse{}, rsConn)
	err = <-errc
	require.Error(t, err)
	require.Equal(t, err, ErrUnexpectedResponse)
}

func testSetupSocketPair(
	t *testing.T,
	chainID string,
	privValidator types.PrivValidator,
) (*SocketPV, *RemoteSigner) {
	var (
		addr    = testFreeAddr(t)
		logger  = log.TestingLogger()
		privVal = privValidator
		readyc  = make(chan struct{})
		rs      = NewRemoteSigner(
			logger,
			chainID,
			addr,
			privVal,
			ed25519.GenPrivKey(),
		)
		sc = NewSocketPV(
			logger,
			addr,
			ed25519.GenPrivKey(),
		)
	)

	testStartSocketPV(t, readyc, sc)

	RemoteSignerConnDeadline(time.Millisecond)(rs)
	RemoteSignerConnRetries(1e6)(rs)

	require.NoError(t, rs.Start())
	assert.True(t, rs.IsRunning())

	<-readyc

	return sc, rs
}

func testReadWriteResponse(t *testing.T, resp SocketPVMsg, rsConn net.Conn) {
	_, err := readMsg(rsConn)
	require.NoError(t, err)

	err = writeMsg(rsConn, resp)
	require.NoError(t, err)
}

func testStartSocketPV(t *testing.T, readyc chan struct{}, sc *SocketPV) {
	go func(sc *SocketPV) {
		require.NoError(t, sc.Start())
		assert.True(t, sc.IsRunning())

		readyc <- struct{}{}
	}(sc)
}

// testFreeAddr claims a free port so we don't block on listener being ready.
func testFreeAddr(t *testing.T) string {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	return fmt.Sprintf("127.0.0.1:%d", ln.Addr().(*net.TCPAddr).Port)
}
