package types

import (
	"reflect"
	"testing"

	crypto "github.com/tendermint/go-crypto"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tmlibs/log"
)

func TestPrivValidatorSocketServer(t *testing.T) {
	var (
		chainID = "test-chain"
		logger  = log.TestingLogger()
		signer  = types.GenSigner()
		privKey = crypto.GenPrivKeyEd25519()
		privVal = NewTestPrivValidator(signer)
		pvss    = NewPrivValidatorSocketServer(
			logger,
			"127.0.0.1:0",
			chainID,
			privVal,
			privKey,
			1,
		)
	)

	err := pvss.Start()
	if err != nil {
		t.Fatal(err)
	}

	c := NewPrivValidatorSocketClient(logger, pvss.listener.Addr().String())

	err = c.Start()
	if err != nil {
		t.Fatal(err)
	}

	if have, want := c.PubKey(), pvss.privVal.PubKey(); !reflect.DeepEqual(have, want) {
		t.Errorf("have %v, want %v", have, want)
	}
}
