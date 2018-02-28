package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	crypto "github.com/tendermint/go-crypto"
	"github.com/tendermint/tmlibs/log"

	"github.com/tendermint/tendermint/types"
)

func TestSocketClientAddress(t *testing.T) {
	var (
		assert, require = assert.New(t), require.New(t)
		chainID         = "test-chain-secret"
		sc, pvss        = testSetupSocketPair(t, chainID)
	)
	defer sc.Stop()
	defer pvss.Stop()

	serverAddr, err := pvss.privVal.Address()
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
		chainID         = "test-chain-secret"
		sc, pvss        = testSetupSocketPair(t, chainID)
	)
	defer sc.Stop()
	defer pvss.Stop()

	clientKey, err := sc.PubKey()
	require.NoError(err)

	privKey, err := pvss.privVal.PubKey()
	require.NoError(err)

	assert.Equal(privKey, clientKey)

	// TODO(xla): Remove when PrivValidator2 replaced PrivValidator.
	assert.Equal(privKey, sc.GetPubKey())
}

func TestSocketClientProposal(t *testing.T) {
	var (
		assert, require = assert.New(t), require.New(t)
		chainID         = "test-chain-secret"
		sc, pvss        = testSetupSocketPair(t, chainID)

		ts             = time.Now()
		privProposal   = &types.Proposal{Timestamp: ts}
		clientProposal = &types.Proposal{Timestamp: ts}
	)
	defer sc.Stop()
	defer pvss.Stop()

	require.NoError(pvss.privVal.SignProposal(chainID, privProposal))
	require.NoError(sc.SignProposal(chainID, clientProposal))
	assert.Equal(privProposal.Signature, clientProposal.Signature)
}

func TestSocketClientVote(t *testing.T) {
	var (
		assert, require = assert.New(t), require.New(t)
		chainID         = "test-chain-secret"
		sc, pvss        = testSetupSocketPair(t, chainID)

		ts    = time.Now()
		vType = types.VoteTypePrecommit
		want  = &types.Vote{Timestamp: ts, Type: vType}
		have  = &types.Vote{Timestamp: ts, Type: vType}
	)
	defer sc.Stop()
	defer pvss.Stop()

	require.NoError(pvss.privVal.SignVote(chainID, want))
	require.NoError(sc.SignVote(chainID, have))
	assert.Equal(want.Signature, have.Signature)
}

func TestSocketClientHeartbeat(t *testing.T) {
	var (
		assert, require = assert.New(t), require.New(t)
		chainID         = "test-chain-secret"
		sc, pvss        = testSetupSocketPair(t, chainID)

		want = &types.Heartbeat{}
		have = &types.Heartbeat{}
	)
	defer sc.Stop()
	defer pvss.Stop()

	require.NoError(pvss.privVal.SignHeartbeat(chainID, want))
	require.NoError(sc.SignHeartbeat(chainID, have))
	assert.Equal(want.Signature, have.Signature)
}

func TestSocketClientConnectRetryMax(t *testing.T) {
	var (
		assert, _     = assert.New(t), require.New(t)
		logger        = log.TestingLogger()
		clientPrivKey = crypto.GenPrivKeyEd25519()
		sc            = NewSocketClient(
			logger,
			"127.0.0.1:0",
			&clientPrivKey,
		)
	)
	defer sc.Stop()

	SocketClientTimeout(time.Millisecond)(sc)

	assert.EqualError(sc.Start(), ErrDialRetryMax.Error())
}

func testSetupSocketPair(t *testing.T, chainID string) (*socketClient, *PrivValidatorSocketServer) {
	var (
		assert, require = assert.New(t), require.New(t)
		logger          = log.TestingLogger()
		signer          = types.GenSigner()
		clientPrivKey   = crypto.GenPrivKeyEd25519()
		serverPrivKey   = crypto.GenPrivKeyEd25519()
		privVal         = NewTestPrivValidator(signer)
		pvss            = NewPrivValidatorSocketServer(
			logger,
			chainID,
			"127.0.0.1:0",
			1,
			privVal,
			&serverPrivKey,
		)
	)

	err := pvss.Start()
	require.NoError(err)
	assert.True(pvss.IsRunning())

	sc := NewSocketClient(
		logger,
		pvss.listener.Addr().String(),
		&clientPrivKey,
	)

	err = sc.Start()
	require.NoError(err)
	assert.True(sc.IsRunning())

	return sc, pvss
}
