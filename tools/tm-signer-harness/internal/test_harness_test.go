package internal

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	tmjson "github.com/tendermint/tendermint/libs/json"
	"github.com/tendermint/tendermint/libs/log"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/privval"
	"github.com/tendermint/tendermint/types"
)

const (
	defaultConnDeadline = 100
)

func TestRemoteSignerTestHarnessMaxAcceptRetriesReached(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := makeConfig(t, 1, 2)
	defer cleanup(cfg)

	th, err := NewTestHarness(ctx, log.TestingLogger(), cfg)
	require.NoError(t, err)
	th.Run()
	assert.Equal(t, ErrMaxAcceptRetriesReached, th.exitCode)
}

func TestRemoteSignerTestHarnessSuccessfulRun(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	harnessTest(
		ctx,
		t,
		func(th *TestHarness) *privval.SignerServer {
			return newMockSignerServer(t, th, th.fpv.Key.PrivKey, false, false)
		},
		NoError,
	)
}

func TestRemoteSignerPublicKeyCheckFailed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	harnessTest(
		ctx,
		t,
		func(th *TestHarness) *privval.SignerServer {
			return newMockSignerServer(t, th, ed25519.GenPrivKey(), false, false)
		},
		ErrTestPublicKeyFailed,
	)
}

func TestRemoteSignerProposalSigningFailed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	harnessTest(
		ctx,
		t,
		func(th *TestHarness) *privval.SignerServer {
			return newMockSignerServer(t, th, th.fpv.Key.PrivKey, true, false)
		},
		ErrTestSignProposalFailed,
	)
}

func TestRemoteSignerVoteSigningFailed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	harnessTest(
		ctx,
		t,
		func(th *TestHarness) *privval.SignerServer {
			return newMockSignerServer(t, th, th.fpv.Key.PrivKey, false, true)
		},
		ErrTestSignVoteFailed,
	)
}

func newMockSignerServer(
	t *testing.T,
	th *TestHarness,
	privKey crypto.PrivKey,
	breakProposalSigning bool,
	breakVoteSigning bool,
) *privval.SignerServer {
	mockPV := types.NewMockPVWithParams(privKey, breakProposalSigning, breakVoteSigning)

	dialerEndpoint := privval.NewSignerDialerEndpoint(
		th.logger,
		privval.DialTCPFn(
			th.addr,
			time.Duration(defaultConnDeadline)*time.Millisecond,
			ed25519.GenPrivKey(),
		),
	)

	return privval.NewSignerServer(dialerEndpoint, th.chainID, mockPV)
}

// For running relatively standard tests.
func harnessTest(
	ctx context.Context,
	t *testing.T,
	signerServerMaker func(th *TestHarness) *privval.SignerServer,
	expectedExitCode int,
) {
	cfg := makeConfig(t, 100, 3)
	defer cleanup(cfg)

	th, err := NewTestHarness(ctx, log.TestingLogger(), cfg)
	require.NoError(t, err)
	donec := make(chan struct{})
	go func() {
		defer close(donec)
		th.Run()
	}()

	ss := signerServerMaker(th)
	require.NoError(t, ss.Start(ctx))
	assert.True(t, ss.IsRunning())
	defer ss.Stop() //nolint:errcheck // ignore for tests

	<-donec
	assert.Equal(t, expectedExitCode, th.exitCode)
}

func makeConfig(t *testing.T, acceptDeadline, acceptRetries int) TestHarnessConfig {
	t.Helper()
	const keyFilename = "tm-testharness-keyfile"
	const stateFilename = "tm-testharness-statefile"
	pvFile, err := privval.GenFilePV(keyFilename, stateFilename, types.ABCIPubKeyTypeEd25519)
	if err != nil {
		panic(err)
	}
	pvGenDoc := types.GenesisDoc{
		ChainID:         fmt.Sprintf("test-chain-%v", tmrand.Str(6)),
		GenesisTime:     time.Now(),
		ConsensusParams: types.DefaultConsensusParams(),
		Validators: []types.GenesisValidator{
			{
				Address: pvFile.Key.Address,
				PubKey:  pvFile.Key.PubKey,
				Power:   10,
			},
		},
	}

	keyFileContents, err := tmjson.Marshal(pvFile.Key)
	require.NoError(t, err)
	stateFileContents, err := tmjson.Marshal(pvFile.LastSignState)
	require.NoError(t, err)
	genesisFileContents, err := tmjson.Marshal(pvGenDoc)
	require.NoError(t, err)
	return TestHarnessConfig{
		BindAddr:         privval.GetFreeLocalhostAddrPort(),
		KeyFile:          makeTempFile(keyFilename, keyFileContents),
		StateFile:        makeTempFile(stateFilename, stateFileContents),
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

func makeTempFile(name string, content []byte) string {
	tempFile, err := os.CreateTemp("", fmt.Sprintf("%s-*", name))
	if err != nil {
		panic(err)
	}
	if _, err := tempFile.Write(content); err != nil {
		tempFile.Close()
		panic(err)
	}
	if err := tempFile.Close(); err != nil {
		panic(err)
	}
	return tempFile.Name()
}
