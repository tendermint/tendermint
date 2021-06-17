package internal

import (
	"fmt"
	"github.com/dashevo/dashd-go/btcjson"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/tendermint/tendermint/crypto/bls12381"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/privval"
	"github.com/tendermint/tendermint/types"
)

const (
	keyFileContents = `{
	"address": "DDAD59BB10A10088C5A9CA219C3CF5BB4599B54E",
  	"pub_key": {
    	"type": "tendermint/PubKeyBLS12381",
    	"value": "F5BjXeh0DppqaxX7a3LzoWr6CXPZcZeba6VHYdbiUCxQ23b00mFD8FRZpCz9Ug1E"
  	},
  	"priv_key": {
    	"type": "tendermint/PrivKeyBLS12381",
    	"value": "RokcLOxJWTyBkh5HPbdIACng/B65M8a5PYH1Nw6xn70="
  	},
	"pro_tx_hash": "51BF39CC1F41B9FC63DFA5B1EDF3F0CA3AD5CAFAE4B12B4FE9263B08BB50C45F"
}`

	stateFileContents = `{
	"height": "0",
	"round": 0,
	"step": 0
}`

	genesisFileContents = `{
	"genesis_time": "2019-01-15T11:56:34.8963Z",
	"chain_id": "test-chain-0XwP5E",
	"consensus_params": {
		"block": {
			"max_bytes": "22020096",
			"max_gas": "-1",
			"time_iota_ms": "1000"
		},
		"evidence": {
			"max_age_num_blocks": "100000",
			"max_age_duration": "172800000000000",
			"max_num": 50
		},
		"validator": {
			"pub_key_types": [
				"bls12381"
			]
		}
	},
	"validators": [
		{
		"address": "DDAD59BB10A10088C5A9CA219C3CF5BB4599B54E",
		"pub_key": {
			"type": "tendermint/PubKeyBLS12381",
			"value": "F5BjXeh0DppqaxX7a3LzoWr6CXPZcZeba6VHYdbiUCxQ23b00mFD8FRZpCz9Ug1E"
		},
		"power": "100",
		"name": "",
		"pro_tx_hash": "51BF39CC1F41B9FC63DFA5B1EDF3F0CA3AD5CAFAE4B12B4FE9263B08BB50C45F"
		}
	],
    "quorum_hash": "444F39CC1F41B9FC63DFA5B1EDF3F0CA3AD5CAFAE4B12B4FE9263B08BB50C433",
    "threshold_public_key": {
		"type": "tendermint/PubKeyBLS12381",
        "value": "F5BjXeh0DppqaxX7a3LzoWr6CXPZcZeba6VHYdbiUCxQ23b00mFD8FRZpCz9Ug1E"
    },
	"app_hash": ""
}`

	defaultConnDeadline = 100
)

func TestRemoteSignerTestHarnessMaxAcceptRetriesReached(t *testing.T) {
	cfg := makeConfig(t, 1, 2)
	defer cleanup(cfg)

	th, err := NewTestHarness(log.TestingLogger(), cfg)
	require.NoError(t, err)
	th.Run()
	assert.Equal(t, ErrMaxAcceptRetriesReached, th.exitCode)
}

func TestRemoteSignerTestHarnessSuccessfulRun(t *testing.T) {
	harnessTest(
		t,
		func(th *TestHarness) *privval.SignerServer {
			return newMockSignerServer(t, th, th.fpv.Key.PrivKey, th.fpv.Key.ProTxHash, false, false)
		},
		NoError,
	)
}

func TestRemoteSignerPublicKeyCheckFailed(t *testing.T) {
	harnessTest(
		t,
		func(th *TestHarness) *privval.SignerServer {
			return newMockSignerServer(t, th, bls12381.GenPrivKey(), crypto.CRandBytes(32), false, false)
		},
		ErrTestPublicKeyFailed,
	)
}

func TestRemoteSignerProposalSigningFailed(t *testing.T) {
	harnessTest(
		t,
		func(th *TestHarness) *privval.SignerServer {
			return newMockSignerServer(t, th, th.fpv.Key.PrivKey, th.fpv.Key.ProTxHash, true, false)
		},
		ErrTestSignProposalFailed,
	)
}

func TestRemoteSignerVoteSigningFailed(t *testing.T) {
	harnessTest(
		t,
		func(th *TestHarness) *privval.SignerServer {
			return newMockSignerServer(t, th, th.fpv.Key.PrivKey, th.fpv.Key.ProTxHash, false, true)
		},
		ErrTestSignVoteFailed,
	)
}

func newMockSignerServer(
	t *testing.T,
	th *TestHarness,
	privKey crypto.PrivKey,
	proTxHash crypto.ProTxHash,
	breakProposalSigning bool,
	breakVoteSigning bool,
) *privval.SignerServer {
	mockPV := types.NewMockPVWithParams(privKey, proTxHash, breakProposalSigning, breakVoteSigning)

	dialerEndpoint := privval.NewSignerDialerEndpoint(
		th.logger,
		privval.DialTCPFn(
			th.addr,
			time.Duration(defaultConnDeadline)*time.Millisecond,
			ed25519.GenPrivKey(),
		),
	)

	return privval.NewSignerServer(dialerEndpoint, th.chainID, btcjson.LLMQType_5_60, crypto.RandQuorumHash(), mockPV)
}

// For running relatively standard tests.
func harnessTest(t *testing.T, signerServerMaker func(th *TestHarness) *privval.SignerServer, expectedExitCode int) {
	cfg := makeConfig(t, 100, 3)
	defer cleanup(cfg)

	th, err := NewTestHarness(log.TestingLogger(), cfg)
	require.NoError(t, err)
	donec := make(chan struct{})
	go func() {
		defer close(donec)
		th.Run()
	}()

	ss := signerServerMaker(th)
	require.NoError(t, ss.Start())
	assert.True(t, ss.IsRunning())
	defer ss.Stop() //nolint:errcheck // ignore for tests

	<-donec
	assert.Equal(t, expectedExitCode, th.exitCode)
}

func makeConfig(t *testing.T, acceptDeadline, acceptRetries int) TestHarnessConfig {
	return TestHarnessConfig{
		BindAddr:         privval.GetFreeLocalhostAddrPort(),
		KeyFile:          makeTempFile("tm-testharness-keyfile", keyFileContents),
		StateFile:        makeTempFile("tm-testharness-statefile", stateFileContents),
		GenesisFile:      makeTempFile("tm-testharness-genesisfile", genesisFileContents),
		AcceptDeadline:   time.Duration(acceptDeadline) * time.Millisecond,
		ConnDeadline:     time.Duration(defaultConnDeadline) * time.Millisecond,
		AcceptRetries:    acceptRetries,
		SecretConnKey:    ed25519.GenPrivKey(),
		ExitWhenComplete: false,
	}
}

func cleanup(cfg TestHarnessConfig) {
	os.Remove(cfg.KeyFile)
	os.Remove(cfg.StateFile)
	os.Remove(cfg.GenesisFile)
}

func makeTempFile(name, content string) string {
	tempFile, err := ioutil.TempFile("", fmt.Sprintf("%s-*", name))
	if err != nil {
		panic(err)
	}
	if _, err := tempFile.Write([]byte(content)); err != nil {
		tempFile.Close()
		panic(err)
	}
	if err := tempFile.Close(); err != nil {
		panic(err)
	}
	return tempFile.Name()
}
