package internal

import (
	"bytes"
	"fmt"
	"github.com/dashevo/dashd-go/btcjson"
	"net"
	"os"
	"os/signal"
	"time"

	"github.com/tendermint/tendermint/crypto"

	"github.com/tendermint/tendermint/crypto/ed25519"

	"github.com/tendermint/tendermint/crypto/tmhash"

	"github.com/tendermint/tendermint/privval"
	"github.com/tendermint/tendermint/state"

	"github.com/tendermint/tendermint/libs/log"
	tmnet "github.com/tendermint/tendermint/libs/net"
	tmos "github.com/tendermint/tendermint/libs/os"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

// Test harness error codes (which act as exit codes when the test harness fails).
const (
	NoError                    int = iota // 0
	ErrInvalidParameters                  // 1
	ErrMaxAcceptRetriesReached            // 2
	ErrFailedToLoadGenesisFile            // 3
	ErrFailedToCreateListener             // 4
	ErrFailedToStartListener              // 5
	ErrInterrupted                        // 6
	ErrOther                              // 7
	ErrTestPublicKeyFailed                // 8
	ErrTestSignProposalFailed             // 9
	ErrTestSignVoteFailed                 // 10
)

var voteTypes = []tmproto.SignedMsgType{tmproto.PrevoteType, tmproto.PrecommitType}

// TestHarnessError allows us to keep track of which exit code should be used
// when exiting the main program.
type TestHarnessError struct {
	Code int    // The exit code to return
	Err  error  // The original error
	Info string // Any additional information
}

var _ error = (*TestHarnessError)(nil)

// TestHarness allows for testing of a remote signer to ensure compatibility
// with this version of Tendermint.
type TestHarness struct {
	addr             string
	signerClient     *privval.SignerClient
	fpv              *privval.FilePV
	chainID          string
	quorumHash       crypto.QuorumHash
	acceptRetries    int
	logger           log.Logger
	exitWhenComplete bool
	exitCode         int
}

// TestHarnessConfig provides configuration to set up a remote signer test
// harness.
type TestHarnessConfig struct {
	BindAddr string

	KeyFile     string
	StateFile   string
	GenesisFile string

	AcceptDeadline time.Duration
	ConnDeadline   time.Duration
	AcceptRetries  int

	SecretConnKey ed25519.PrivKey

	ExitWhenComplete bool // Whether or not to call os.Exit when the harness has completed.
}

// timeoutError can be used to check if an error returned from the netp package
// was due to a timeout.
type timeoutError interface {
	Timeout() bool
}

// NewTestHarness will load Tendermint data from the given files (including
// validator public/private keypairs and chain details) and create a new
// harness.
func NewTestHarness(logger log.Logger, cfg TestHarnessConfig) (*TestHarness, error) {
	keyFile := ExpandPath(cfg.KeyFile)
	stateFile := ExpandPath(cfg.StateFile)
	logger.Info("Loading private validator configuration", "keyFile", keyFile, "stateFile", stateFile)
	// NOTE: LoadFilePV ultimately calls os.Exit on failure. No error will be
	// returned if this call fails.
	fpv := privval.LoadFilePV(keyFile, stateFile)

	genesisFile := ExpandPath(cfg.GenesisFile)
	logger.Info("Loading chain ID from genesis file", "genesisFile", genesisFile)
	st, err := state.MakeGenesisDocFromFile(genesisFile)
	if err != nil {
		return nil, newTestHarnessError(ErrFailedToLoadGenesisFile, err, genesisFile)
	}
	logger.Info("Loaded genesis file", "chainID", st.ChainID)

	spv, err := newTestHarnessListener(logger, cfg)
	if err != nil {
		return nil, newTestHarnessError(ErrFailedToCreateListener, err, "")
	}

	signerClient, err := privval.NewSignerClient(spv, st.ChainID)
	if err != nil {
		return nil, newTestHarnessError(ErrFailedToCreateListener, err, "")
	}

	return &TestHarness{
		addr:             cfg.BindAddr,
		signerClient:     signerClient,
		fpv:              fpv,
		chainID:          st.ChainID,
		quorumHash:       st.QuorumHash,
		acceptRetries:    cfg.AcceptRetries,
		logger:           logger,
		exitWhenComplete: cfg.ExitWhenComplete,
		exitCode:         0,
	}, nil
}

// Run will execute the tests associated with this test harness. The intention
// here is to call this from one's `main` function, as the way it succeeds or
// fails at present is to call os.Exit() with an exit code related to the error
// that caused the tests to fail, or exit code 0 on success.
func (th *TestHarness) Run() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for sig := range c {
			th.logger.Info("Caught interrupt, terminating...", "sig", sig)
			th.Shutdown(newTestHarnessError(ErrInterrupted, nil, ""))
		}
	}()

	th.logger.Info("Starting test harness")
	accepted := false
	var startErr error

	for acceptRetries := th.acceptRetries; acceptRetries > 0; acceptRetries-- {
		th.logger.Info("Attempting to accept incoming connection", "acceptRetries", acceptRetries)

		if err := th.signerClient.WaitForConnection(10 * time.Millisecond); err != nil {
			// if it wasn't a timeout error
			if _, ok := err.(timeoutError); !ok {
				th.logger.Error("Failed to start listener", "err", err)
				th.Shutdown(newTestHarnessError(ErrFailedToStartListener, err, ""))
				// we need the return statements in case this is being run
				// from a unit test - otherwise this function will just die
				// when os.Exit is called
				return
			}
			startErr = err
		} else {
			th.logger.Info("Accepted external connection")
			accepted = true
			break
		}
	}
	if !accepted {
		th.logger.Error("Maximum accept retries reached", "acceptRetries", th.acceptRetries)
		th.Shutdown(newTestHarnessError(ErrMaxAcceptRetriesReached, startErr, ""))
		return
	}

	// Run the tests
	if err := th.TestPublicKey(); err != nil {
		th.Shutdown(err)
		return
	}
	if err := th.TestSignProposal(); err != nil {
		th.Shutdown(err)
		return
	}
	if err := th.TestSignVote(); err != nil {
		th.Shutdown(err)
		return
	}
	th.logger.Info("SUCCESS! All tests passed.")
	th.Shutdown(nil)
}

// TestPublicKey just validates that we can (1) fetch the public key from the
// remote signer, and (2) it matches the public key we've configured for our
// local Tendermint version.
func (th *TestHarness) TestPublicKey() error {
	th.logger.Info("TEST: Public key of remote signer")
	fpvk, err := th.fpv.GetPubKey(th.quorumHash)
	if err != nil {
		return err
	}
	th.logger.Info("Local", "pubKey", fpvk)
	sck, err := th.signerClient.GetPubKey(th.quorumHash)
	if err != nil {
		return err
	}
	th.logger.Info("Remote", "pubKey", sck)
	if !bytes.Equal(fpvk.Bytes(), sck.Bytes()) {
		th.logger.Error("FAILED: Local and remote public keys do not match")
		return newTestHarnessError(ErrTestPublicKeyFailed, nil, "")
	}
	return nil
}

// TestSignProposal makes sure the remote signer can successfully sign
// proposals.
func (th *TestHarness) TestSignProposal() error {
	th.logger.Info("TEST: Signing of proposals")
	// sha256 hash of "hash"
	hash := tmhash.Sum([]byte("hash"))
	prop := &types.Proposal{
		Type:                  tmproto.ProposalType,
		Height:                100,
		CoreChainLockedHeight: 1,
		Round:                 0,
		POLRound:              -1,
		BlockID: types.BlockID{
			Hash: hash,
			PartSetHeader: types.PartSetHeader{
				Hash:  hash,
				Total: 1000000,
			},
		},
		Timestamp: time.Now(),
	}
	p := prop.ToProto()
	propSignId := types.ProposalBlockSignId(th.chainID, p, btcjson.LLMQType_5_60, th.quorumHash)
	if err := th.signerClient.SignProposal(th.chainID, btcjson.LLMQType_5_60, th.quorumHash, p); err != nil {
		th.logger.Error("FAILED: Signing of proposal", "err", err)
		return newTestHarnessError(ErrTestSignProposalFailed, err, "")
	}
	prop.Signature = p.Signature
	th.logger.Debug("Signed proposal", "prop", prop)
	// first check that it's a basically valid proposal
	if err := prop.ValidateBasic(); err != nil {
		th.logger.Error("FAILED: Signed proposal is invalid", "err", err)
		return newTestHarnessError(ErrTestSignProposalFailed, err, "")
	}
	sck, err := th.signerClient.GetPubKey(th.quorumHash)
	if err != nil {
		return err
	}
	// now validate the signature on the proposal
	if sck.VerifySignatureDigest(propSignId, prop.Signature) {
		th.logger.Info("Successfully validated proposal signature")
	} else {
		th.logger.Error("FAILED: Proposal signature validation failed")
		return newTestHarnessError(ErrTestSignProposalFailed, nil, "signature validation failed")
	}
	return nil
}

// TestSignVote makes sure the remote signer can successfully sign all kinds of
// votes.
func (th *TestHarness) TestSignVote() error {
	th.logger.Info("TEST: Signing of votes")
	for _, voteType := range voteTypes {
		th.logger.Info("Testing vote type", "type", voteType)
		hash := tmhash.Sum([]byte("hash"))
		lastAppHash := tmhash.Sum([]byte("hash"))
		vote := &types.Vote{
			Type:   voteType,
			Height: 101,
			Round:  0,
			BlockID: types.BlockID{
				Hash: hash,
				PartSetHeader: types.PartSetHeader{
					Hash:  hash,
					Total: 1000000,
				},
			},
			StateID: types.StateID{
				LastAppHash: lastAppHash,
			},
			ValidatorIndex:     0,
			ValidatorProTxHash: tmhash.Sum([]byte("pro_tx_hash")),
		}
		v := vote.ToProto()
		voteBlockId := types.VoteBlockSignId(th.chainID, v, btcjson.LLMQType_5_60, th.quorumHash)
		voteStateId := types.VoteStateSignId(th.chainID, v, btcjson.LLMQType_5_60, th.quorumHash)
		// sign the vote
		if err := th.signerClient.SignVote(th.chainID, btcjson.LLMQType_5_60, th.quorumHash, v); err != nil {
			th.logger.Error("FAILED: Signing of vote", "err", err)
			return newTestHarnessError(ErrTestSignVoteFailed, err, fmt.Sprintf("voteType=%d", voteType))
		}
		vote.BlockSignature = v.BlockSignature
		vote.StateSignature = v.StateSignature
		th.logger.Debug("Signed vote", "vote", vote)
		// validate the contents of the vote
		if err := vote.ValidateBasic(); err != nil {
			th.logger.Error("FAILED: Signed vote is invalid", "err", err)
			return newTestHarnessError(ErrTestSignVoteFailed, err, fmt.Sprintf("voteType=%d", voteType))
		}
		sck, err := th.signerClient.GetPubKey(th.quorumHash)
		if err != nil {
			return err
		}

		// now validate the signature on the proposal
		if sck.VerifySignatureDigest(voteBlockId, vote.BlockSignature) {
			th.logger.Info("Successfully validated vote signature", "type", voteType)
		} else {
			th.logger.Error("FAILED: Vote signature validation failed", "type", voteType)
			return newTestHarnessError(ErrTestSignVoteFailed, nil, "signature validation failed")
		}

		if sck.VerifySignatureDigest(voteStateId, vote.StateSignature) {
			th.logger.Info("Successfully validated vote signature", "type", voteType)
		} else {
			th.logger.Error("FAILED: Vote signature validation failed", "type", voteType)
			return newTestHarnessError(ErrTestSignVoteFailed, nil, "signature validation failed")
		}
	}
	return nil
}

// Shutdown will kill the test harness and attempt to close all open sockets
// gracefully. If the supplied error is nil, it is assumed that the exit code
// should be 0. If err is not nil, it will exit with an exit code related to the
// error.
func (th *TestHarness) Shutdown(err error) {
	var exitCode int

	if err == nil {
		exitCode = NoError
	} else if therr, ok := err.(*TestHarnessError); ok {
		exitCode = therr.Code
	} else {
		exitCode = ErrOther
	}
	th.exitCode = exitCode

	// in case sc.Stop() takes too long
	if th.exitWhenComplete {
		go func() {
			time.Sleep(time.Duration(5) * time.Second)
			th.logger.Error("Forcibly exiting program after timeout")
			os.Exit(exitCode)
		}()
	}

	err = th.signerClient.Close()
	if err != nil {
		th.logger.Error("Failed to cleanly stop listener: %s", err.Error())
	}

	if th.exitWhenComplete {
		os.Exit(exitCode)
	}
}

// newTestHarnessListener creates our client instance which we will use for testing.
func newTestHarnessListener(logger log.Logger, cfg TestHarnessConfig) (*privval.SignerListenerEndpoint, error) {
	proto, addr := tmnet.ProtocolAndAddress(cfg.BindAddr)
	if proto == "unix" {
		// make sure the socket doesn't exist - if so, try to delete it
		if tmos.FileExists(addr) {
			if err := os.Remove(addr); err != nil {
				logger.Error("Failed to remove existing Unix domain socket", "addr", addr)
				return nil, err
			}
		}
	}
	ln, err := net.Listen(proto, addr)
	if err != nil {
		return nil, err
	}
	logger.Info("Listening", "proto", proto, "addr", addr)
	var svln net.Listener
	switch proto {
	case "unix":
		unixLn := privval.NewUnixListener(ln)
		privval.UnixListenerTimeoutAccept(cfg.AcceptDeadline)(unixLn)
		privval.UnixListenerTimeoutReadWrite(cfg.ConnDeadline)(unixLn)
		svln = unixLn
	case "tcp":
		tcpLn := privval.NewTCPListener(ln, cfg.SecretConnKey)
		privval.TCPListenerTimeoutAccept(cfg.AcceptDeadline)(tcpLn)
		privval.TCPListenerTimeoutReadWrite(cfg.ConnDeadline)(tcpLn)
		logger.Info("Resolved TCP address for listener", "addr", tcpLn.Addr())
		svln = tcpLn
	default:
		_ = ln.Close()
		logger.Error("Unsupported protocol (must be unix:// or tcp://)", "proto", proto)
		return nil, newTestHarnessError(ErrInvalidParameters, nil, fmt.Sprintf("Unsupported protocol: %s", proto))
	}
	return privval.NewSignerListenerEndpoint(logger, svln), nil
}

func newTestHarnessError(code int, err error, info string) *TestHarnessError {
	return &TestHarnessError{
		Code: code,
		Err:  err,
		Info: info,
	}
}

func (e *TestHarnessError) Error() string {
	var msg string
	switch e.Code {
	case ErrInvalidParameters:
		msg = "Invalid parameters supplied to application"
	case ErrMaxAcceptRetriesReached:
		msg = "Maximum accept retries reached"
	case ErrFailedToLoadGenesisFile:
		msg = "Failed to load genesis file"
	case ErrFailedToCreateListener:
		msg = "Failed to create listener"
	case ErrFailedToStartListener:
		msg = "Failed to start listener"
	case ErrInterrupted:
		msg = "Interrupted"
	case ErrTestPublicKeyFailed:
		msg = "Public key validation test failed"
	case ErrTestSignProposalFailed:
		msg = "Proposal signing validation test failed"
	case ErrTestSignVoteFailed:
		msg = "Vote signing validation test failed"
	default:
		msg = "Unknown error"
	}
	if len(e.Info) > 0 {
		msg = fmt.Sprintf("%s: %s", msg, e.Info)
	}
	if e.Err != nil {
		msg = fmt.Sprintf("%s (original error: %s)", msg, e.Err.Error())
	}
	return msg
}
