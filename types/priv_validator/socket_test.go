package types

import (
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	crypto "github.com/tendermint/go-crypto"
	cmn "github.com/tendermint/tmlibs/common"
	"github.com/tendermint/tmlibs/log"

	"github.com/tendermint/tendermint/types"
)

func TestSocketClientAddress(t *testing.T) {
	var (
		assert, require = assert.New(t), require.New(t)
		chainID         = cmn.RandStr(12)
		sc, rs          = testSetupSocketPair(t, chainID)
	)
	defer sc.Stop()
	defer rs.Stop()

	serverAddr, err := rs.privVal.Address()
	require.NoError(err)

	clientAddr, err := sc.Address()
	require.NoError(err)

	assert.Equal(serverAddr, clientAddr)

	// TODO(xla): Remove when PrivValidator2 replaced PrivValidator.
	assert.Equal(serverAddr, sc.GetAddress())

}

func TestSocketClientPubKey(t *testing.T) {
	var (
		assert, require = assert.New(t), require.New(t)
		chainID         = cmn.RandStr(12)
		sc, rs          = testSetupSocketPair(t, chainID)
	)
	defer sc.Stop()
	defer rs.Stop()

	clientKey, err := sc.PubKey()
	require.NoError(err)

	privKey, err := rs.privVal.PubKey()
	require.NoError(err)

	assert.Equal(privKey, clientKey)

	// TODO(xla): Remove when PrivValidator2 replaced PrivValidator.
	assert.Equal(privKey, sc.GetPubKey())
}

func TestSocketClientProposal(t *testing.T) {
	var (
		assert, require = assert.New(t), require.New(t)
		chainID         = cmn.RandStr(12)
		sc, rs          = testSetupSocketPair(t, chainID)

		ts             = time.Now()
		privProposal   = &types.Proposal{Timestamp: ts}
		clientProposal = &types.Proposal{Timestamp: ts}
	)
	defer sc.Stop()
	defer rs.Stop()

	require.NoError(rs.privVal.SignProposal(chainID, privProposal))
	require.NoError(sc.SignProposal(chainID, clientProposal))
	assert.Equal(privProposal.Signature, clientProposal.Signature)
}

func TestSocketClientVote(t *testing.T) {
	var (
		assert, require = assert.New(t), require.New(t)
		chainID         = cmn.RandStr(12)
		sc, rs          = testSetupSocketPair(t, chainID)

		ts    = time.Now()
		vType = types.VoteTypePrecommit
		want  = &types.Vote{Timestamp: ts, Type: vType}
		have  = &types.Vote{Timestamp: ts, Type: vType}
	)
	defer sc.Stop()
	defer rs.Stop()

	require.NoError(rs.privVal.SignVote(chainID, want))
	require.NoError(sc.SignVote(chainID, have))
	assert.Equal(want.Signature, have.Signature)
}

func TestSocketClientHeartbeat(t *testing.T) {
	var (
		assert, require = assert.New(t), require.New(t)
		chainID         = cmn.RandStr(12)
		sc, rs          = testSetupSocketPair(t, chainID)

		want = &types.Heartbeat{}
		have = &types.Heartbeat{}
	)
	defer sc.Stop()
	defer rs.Stop()

	require.NoError(rs.privVal.SignHeartbeat(chainID, want))
	require.NoError(sc.SignHeartbeat(chainID, have))
	assert.Equal(want.Signature, have.Signature)
}

func TestSocketClientDeadline(t *testing.T) {
	var (
		assert, require = assert.New(t), require.New(t)
		readyc          = make(chan struct{})
		sc              = NewSocketClient(
			log.TestingLogger(),
			"127.0.0.1:0",
			nil,
		)
	)
	defer sc.Stop()

	SocketClientConnDeadline(time.Millisecond)(sc)

	require.NoError(sc.listen())

	go func(sc *SocketClient) {
		require.NoError(sc.Start())
		assert.True(sc.IsRunning())

		readyc <- struct{}{}
	}(sc)

	_, err := cmn.Connect(sc.listener.Addr().String())
	require.NoError(err)

	<-readyc

	_, err = sc.PubKey()
	assert.Equal(errors.Cause(err), ErrConnTimeout)
}

func TestSocketClientWait(t *testing.T) {
	var (
		assert, _ = assert.New(t), require.New(t)
		logger    = log.TestingLogger()
		privKey   = crypto.GenPrivKeyEd25519()
		sc        = NewSocketClient(
			logger,
			"127.0.0.1:0",
			&privKey,
		)
	)
	defer sc.Stop()

	SocketClientConnWait(time.Millisecond)(sc)

	assert.EqualError(sc.Start(), ErrConnWaitTimeout.Error())
}

func TestRemoteSignerRetry(t *testing.T) {
	var (
		assert, _ = assert.New(t), require.New(t)
		privKey   = crypto.GenPrivKeyEd25519()
		rs        = NewRemoteSigner(
			log.TestingLogger(),
			cmn.RandStr(12),
			"127.0.0.1:0",
			NewTestPrivValidator(types.GenSigner()),
			&privKey,
		)
	)
	defer rs.Stop()

	RemoteSignerConnDeadline(time.Millisecond)(rs)
	RemoteSignerConnRetries(2)(rs)

	assert.EqualError(rs.Start(), ErrDialRetryMax.Error())
}

func testSetupSocketPair(
	t *testing.T,
	chainID string,
) (*SocketClient, *RemoteSigner) {
	var (
		assert, require = assert.New(t), require.New(t)
		logger          = log.TestingLogger()
		signer          = types.GenSigner()
		clientPrivKey   = crypto.GenPrivKeyEd25519()
		remotePrivKey   = crypto.GenPrivKeyEd25519()
		privVal         = NewTestPrivValidator(signer)
		readyc          = make(chan struct{})
		sc              = NewSocketClient(
			logger,
			"127.0.0.1:0",
			&clientPrivKey,
		)
	)

	require.NoError(sc.listen())

	go func(sc *SocketClient) {
		require.NoError(sc.Start())
		assert.True(sc.IsRunning())

		readyc <- struct{}{}
	}(sc)

	rs := NewRemoteSigner(
		logger,
		chainID,
		sc.listener.Addr().String(),
		privVal,
		&remotePrivKey,
	)
	require.NoError(rs.Start())
	assert.True(rs.IsRunning())

	<-readyc

	return sc, rs
}
