package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	crypto "github.com/tendermint/go-crypto"
	data "github.com/tendermint/go-wire/data"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tmlibs/log"
)

func TestPrivValidatorSocketServer(t *testing.T) {
	var (
		assert, require = assert.New(t), require.New(t)
		chainID         = "test-chain-secret"
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
	require.Nil(err)
	defer pvss.Stop()

	assert.True(pvss.IsRunning())

	pvsc := NewPrivValidatorSocketClient(
		logger,
		pvss.listener.Addr().String(),
		&clientPrivKey,
	)

	err = pvsc.Start()
	require.Nil(err)
	defer pvsc.Stop()

	assert.True(pvsc.IsRunning())

	assert.Equal(pvsc.Address(), data.Bytes(pvss.privVal.PubKey().Address()))
	assert.Equal(pvsc.PubKey(), pvss.privVal.PubKey())

	err = pvsc.SignProposal(chainID, &types.Proposal{
		Timestamp: time.Now(),
	})
	require.Nil(err)

	err = pvsc.SignVote(chainID, &types.Vote{
		Timestamp: time.Now(),
		Type:      types.VoteTypePrecommit,
	})
	require.Nil(err)

	err = pvsc.SignHeartbeat(chainID, &types.Heartbeat{})
	require.Nil(err)
}

func TestPrivValidatorSocketServerWithoutSecret(t *testing.T) {
	var (
		assert, require = assert.New(t), require.New(t)
		chainID         = "test-chain-secret"
		logger          = log.TestingLogger()
		signer          = types.GenSigner()
		privVal         = NewTestPrivValidator(signer)
		pvss            = NewPrivValidatorSocketServer(
			logger,
			chainID,
			"127.0.0.1:0",
			1,
			privVal,
			nil,
		)
	)

	err := pvss.Start()
	require.Nil(err)
	defer pvss.Stop()

	assert.True(pvss.IsRunning())

	pvsc := NewPrivValidatorSocketClient(
		logger,
		pvss.listener.Addr().String(),
		nil,
	)

	err = pvsc.Start()
	require.Nil(err)
	defer pvsc.Stop()

	assert.True(pvsc.IsRunning())

	assert.Equal(pvsc.Address(), data.Bytes(pvss.privVal.PubKey().Address()))
	assert.Equal(pvsc.PubKey(), pvss.privVal.PubKey())

	err = pvsc.SignProposal(chainID, &types.Proposal{
		Timestamp: time.Now(),
	})
	require.Nil(err)

	err = pvsc.SignVote(chainID, &types.Vote{
		Timestamp: time.Now(),
		Type:      types.VoteTypePrecommit,
	})
	require.Nil(err)

	err = pvsc.SignHeartbeat(chainID, &types.Heartbeat{})
	require.Nil(err)
}
