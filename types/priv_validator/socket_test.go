package types

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	crypto "github.com/tendermint/go-crypto"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tmlibs/log"
)

func TestPrivValidatorSocketServer(t *testing.T) {
	var (
		assert, require = assert.New(t), require.New(t)
		chainID         = "test-chain"
		logger          = log.TestingLogger()
		signer          = types.GenSigner()
		clientPrivKey   = crypto.GenPrivKeyEd25519()
		serverPrivKey   = crypto.GenPrivKeyEd25519()
		privVal         = NewTestPrivValidator(signer)
		pvss            = NewPrivValidatorSocketServer(
			logger,
			"127.0.0.1:0",
			chainID,
			privVal,
			&serverPrivKey,
			1,
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

	if have, want := pvsc.PubKey(), pvss.privVal.PubKey(); !reflect.DeepEqual(have, want) {
		t.Errorf("have %v, want %v", have, want)
	}
}
