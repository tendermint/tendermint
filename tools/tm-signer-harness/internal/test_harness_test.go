package internal

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"testing"
	"time"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/privval"
	"github.com/tendermint/tendermint/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/libs/log"
)

const (
	keyFileContents = `{
	"address": "D08FCA3BA74CF17CBFC15E64F9505302BB0E2748",
	"pub_key": {
		"type": "tendermint/PubKeyEd25519",
		"value": "ZCsuTjaczEyon70nmKxwvwu+jqrbq5OH3yQjcK0SFxc="
	},
	"priv_key": {
		"type": "tendermint/PrivKeyEd25519",
		"value": "8O39AkQsoe1sBQwud/Kdul8lg8K9SFsql9aZvwXQSt1kKy5ONpzMTKifvSeYrHC/C76Oqturk4ffJCNwrRIXFw=="
	}
}`

	stateFileContents = `{
	"height": "0",
	"round": "0",
	"step": 0
}`

	genesisFileContents = `{
	"genesis_time": "2019-01-15T11:56:34.8963Z",
	"chain_id": "test-chain-0XwP5E",
	"consensus_params": {
		"block_size": {
		"max_bytes": "22020096",
		"max_gas": "-1"
		},
		"evidence": {
		"max_age": "100000"
		},
		"validator": {
		"pub_key_types": [
			"ed25519"
		]
		}
	},
	"validators": [
		{
		"address": "D08FCA3BA74CF17CBFC15E64F9505302BB0E2748",
		"pub_key": {
			"type": "tendermint/PubKeyEd25519",
			"value": "ZCsuTjaczEyon70nmKxwvwu+jqrbq5OH3yQjcK0SFxc="
		},
		"power": "10",
		"name": ""
		}
	],
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
		func(th *TestHarness) *privval.RemoteSigner {
			return newMockRemoteSigner(t, th, th.fpv.Key.PrivKey, false, false)
		},
		NoError,
	)
}

func TestRemoteSignerPublicKeyCheckFailed(t *testing.T) {
	harnessTest(
		t,
		func(th *TestHarness) *privval.RemoteSigner {
			return newMockRemoteSigner(t, th, ed25519.GenPrivKey(), false, false)
		},
		ErrTestPublicKeyFailed,
	)
}

func TestRemoteSignerProposalSigningFailed(t *testing.T) {
	harnessTest(
		t,
		func(th *TestHarness) *privval.RemoteSigner {
			return newMockRemoteSigner(t, th, th.fpv.Key.PrivKey, true, false)
		},
		ErrTestSignProposalFailed,
	)
}

func TestRemoteSignerVoteSigningFailed(t *testing.T) {
	harnessTest(
		t,
		func(th *TestHarness) *privval.RemoteSigner {
			return newMockRemoteSigner(t, th, th.fpv.Key.PrivKey, false, true)
		},
		ErrTestSignVoteFailed,
	)
}

func newMockRemoteSigner(t *testing.T, th *TestHarness, privKey crypto.PrivKey, breakProposalSigning bool, breakVoteSigning bool) *privval.RemoteSigner {
	return privval.NewRemoteSigner(
		th.logger,
		th.chainID,
		types.NewMockPVWithParams(privKey, breakProposalSigning, breakVoteSigning),
		privval.DialTCPFn(
			th.addr,
			time.Duration(defaultConnDeadline)*time.Millisecond,
			ed25519.GenPrivKey(),
		),
	)
}

// For running relatively standard tests.
func harnessTest(t *testing.T, rsMaker func(th *TestHarness) *privval.RemoteSigner, expectedExitCode int) {
	cfg := makeConfig(t, 100, 3)
	defer cleanup(cfg)

	th, err := NewTestHarness(log.TestingLogger(), cfg)
	require.NoError(t, err)
	donec := make(chan struct{})
	go func() {
		defer close(donec)
		th.Run()
	}()

	rs := rsMaker(th)
	require.NoError(t, rs.Start())
	assert.True(t, rs.IsRunning())
	defer rs.Stop()

	<-donec
	assert.Equal(t, expectedExitCode, th.exitCode)
}

func makeConfig(t *testing.T, acceptDeadline, acceptRetries int) TestHarnessConfig {
	return TestHarnessConfig{
		BindAddr:         testFreeTCPAddr(t),
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

// testFreeTCPAddr claims a free port so we don't block on listener being ready.
func testFreeTCPAddr(t *testing.T) string {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	return fmt.Sprintf("127.0.0.1:%d", ln.Addr().(*net.TCPAddr).Port)
}
