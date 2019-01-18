package internal

import (
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	amino "github.com/tendermint/go-amino"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/privval"
	"github.com/tendermint/tendermint/state"

	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
)

// Test harness error codes (which act as exit codes when the test harness fails).
const (
	NoError int = iota
	ErrFailedToExpandPath
	ErrFailedToLoadGenesisFile
	ErrFailedToCreateListener
	ErrFailedToStartListener
	ErrInterrupted
	ErrOther
	ErrTestPublicKeyFailed
	ErrTestSignProposalFailed
	ErrTestSignVoteFailed
)

var voteTypes = []types.SignedMsgType{types.PrevoteType, types.PrecommitType}

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
	sc      *privval.SocketVal
	fpv     *privval.FilePV
	chainID string
	logger  log.Logger
}

type TestHarnessConfig struct {
	BindAddr string

	KeyFile     string
	StateFile   string
	GenesisFile string

	AcceptDeadline time.Duration
	ConnDeadline   time.Duration

	SecretConnKey ed25519.PrivKeyEd25519
}

var cdc = amino.NewCodec()

// NewTestHarness will load Tendermint data from the given files (including
// validator public/private keypairs and chain details) and create a new
// harness.
func NewTestHarness(logger log.Logger, cfg TestHarnessConfig) (*TestHarness, error) {
	var err error
	var keyFile, stateFile, genesisFile string

	if keyFile, err = expandPath(cfg.KeyFile); err != nil {
		return nil, newTestHarnessError(ErrFailedToExpandPath, err, cfg.KeyFile)
	}
	if stateFile, err = expandPath(cfg.StateFile); err != nil {
		return nil, newTestHarnessError(ErrFailedToExpandPath, err, cfg.StateFile)
	}
	logger.Info("Loading private validator configuration", "keyFile", keyFile, "stateFile", stateFile)
	// NOTE: LoadFilePV ultimately calls os.Exit on failure. No error will be
	// returned if this call fails.
	fpv := privval.LoadFilePV(keyFile, stateFile)

	if genesisFile, err = expandPath(cfg.GenesisFile); err != nil {
		return nil, newTestHarnessError(ErrFailedToExpandPath, err, cfg.GenesisFile)
	}
	logger.Info("Loading chain ID from genesis file", "genesisFile", genesisFile)
	st, err := state.MakeGenesisDocFromFile(genesisFile)
	if err != nil {
		return nil, newTestHarnessError(ErrFailedToLoadGenesisFile, err, genesisFile)
	}
	logger.Info("Loaded genesis file", "chainID", st.ChainID)

	sc, err := newTestHarnessSocketVal(logger, cfg)
	if err != nil {
		return nil, newTestHarnessError(ErrFailedToCreateListener, err, "")
	}

	return &TestHarness{
		sc:      sc,
		fpv:     fpv,
		chainID: st.ChainID,
		logger:  logger,
	}, nil
}

func (th *TestHarness) Run() {
	th.logger.Info("Starting test harness")
	donec := make(chan struct{})
	go func() {
		defer close(donec)
		if err := th.sc.Start(); err != nil {
			th.logger.Error("Failed to start listener", "err", err)
			th.Shutdown(newTestHarnessError(ErrFailedToStartListener, err, ""))
		}
		if err := th.TestPublicKey(); err != nil {
			th.Shutdown(err)
		}
		if err := th.TestSignProposal(); err != nil {
			th.Shutdown(err)
		}
		if err := th.TestSignVote(); err != nil {
			th.Shutdown(err)
		}
		th.logger.Info("SUCCESS! All tests passed.")
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for sig := range c {
			th.logger.Info("Caught interrupt, terminating...", "sig", sig)
			th.Shutdown(newTestHarnessError(ErrInterrupted, nil, ""))
		}
	}()

	// Run until complete
	<-donec
	th.Shutdown(nil)
}

func (th *TestHarness) TestPublicKey() error {
	th.logger.Info("TEST: Public key of remote signer")
	th.logger.Info("Local", "pubKey", th.fpv.GetPubKey())
	th.logger.Info("Remote", "pubKey", th.sc.GetPubKey())
	if th.fpv.GetPubKey() != th.sc.GetPubKey() {
		th.logger.Error("FAILED: Local and remote public keys do not match")
		return newTestHarnessError(ErrTestPublicKeyFailed, nil, "")
	}
	return nil
}

func (th *TestHarness) TestSignProposal() error {
	th.logger.Info("TEST: Signing of proposals")
	// sha256 hash of "hash"
	hash, _ := hex.DecodeString("D04B98F48E8F8BCC15C6AE5AC050801CD6DCFD428FB5F9E65C4E16E7807340FA")
	prop := &types.Proposal{
		Type:     types.ProposalType,
		Height:   12345,
		Round:    23456,
		POLRound: -1,
		BlockID: types.BlockID{
			Hash: hash,
			PartsHeader: types.PartSetHeader{
				Hash:  hash,
				Total: 1000000,
			},
		},
		Timestamp: time.Now(),
	}
	propBytes, err := cdc.MarshalBinaryLengthPrefixed(types.CanonicalizeProposal(th.chainID, prop))
	if err != nil {
		th.logger.Error("FAILED: Could not marshal proposal to bytes", "err", err)
		return newTestHarnessError(ErrTestSignProposalFailed, err, "")
	}
	if err := th.sc.SignProposal(th.chainID, prop); err != nil {
		th.logger.Error("FAILED: Signing of proposal", "err", err)
		return newTestHarnessError(ErrTestSignProposalFailed, err, "")
	}
	th.logger.Debug("Signed proposal", "prop", prop)
	// first check that it's a basically valid proposal
	if err := prop.ValidateBasic(); err != nil {
		th.logger.Error("FAILED: Signed proposal is invalid", "err", err)
		return newTestHarnessError(ErrTestSignProposalFailed, err, "")
	}
	// now validate the signature on the proposal
	if th.sc.GetPubKey().VerifyBytes(propBytes, prop.Signature) {
		th.logger.Info("Successfully validated proposal signature")
	} else {
		th.logger.Error("FAILED: Proposal signature validation failed")
		return newTestHarnessError(ErrTestSignProposalFailed, err, "signature validation failed")
	}
	return nil
}

func (th *TestHarness) TestSignVote() error {
	th.logger.Info("TEST: Signing of votes")
	for _, voteType := range voteTypes {
		th.logger.Info("Testing vote type", "type", voteType)
		// sha256 hash of "hash"
		hash, _ := hex.DecodeString("D04B98F48E8F8BCC15C6AE5AC050801CD6DCFD428FB5F9E65C4E16E7807340FA")
		vote := &types.Vote{
			Type:   voteType,
			Height: 12345,
			Round:  23456,
			BlockID: types.BlockID{
				Hash: hash,
				PartsHeader: types.PartSetHeader{
					Hash:  hash,
					Total: 1000000,
				},
			},
			ValidatorIndex:   0,
			ValidatorAddress: hash[:20],
			Timestamp:        time.Now(),
		}
		// work out the canonicalized serialized byte form of the message
		// without its signature
		voteBytes, err := cdc.MarshalBinaryLengthPrefixed(vote)
		if err != nil {
			th.logger.Error("FAILED: Could not marshal vote to bytes", "err", err)
			return newTestHarnessError(ErrTestSignVoteFailed, err, fmt.Sprintf("voteType=%d", voteType))
		}
		// sign the vote
		if err := th.sc.SignVote(th.chainID, vote); err != nil {
			th.logger.Error("FAILED: Signing of vote", "err", err)
			return newTestHarnessError(ErrTestSignVoteFailed, err, fmt.Sprintf("voteType=%d", voteType))
		}
		th.logger.Debug("Signed vote", "vote", vote)
		// validate the contents of the vote
		if err := vote.ValidateBasic(); err != nil {
			th.logger.Error("FAILED: Signed vote is invalid", "err", err)
			return newTestHarnessError(ErrTestSignVoteFailed, err, fmt.Sprintf("voteType=%d", voteType))
		}
		// now validate the signature on the proposal
		if th.sc.GetPubKey().VerifyBytes(voteBytes, vote.Signature) {
			th.logger.Info("Successfully validated vote signature", "type", voteType)
		} else {
			th.logger.Error("FAILED: Vote signature validation failed", "type", voteType)
			return newTestHarnessError(ErrTestSignVoteFailed, err, "signature validation failed")
		}
	}
	return nil
}

func (th *TestHarness) Shutdown(err error) {
	var exitCode int

	if err == nil {
		exitCode = NoError
	} else if therr, ok := err.(*TestHarnessError); ok {
		exitCode = therr.Code
	} else {
		exitCode = ErrOther
	}

	// in case sc.Stop() takes too long
	go func() {
		time.Sleep(time.Duration(5) * time.Second)
		th.logger.Error("Forcibly exiting program after timeout")
		os.Exit(exitCode)
	}()

	if th.sc.IsRunning() {
		if err := th.sc.Stop(); err != nil {
			th.logger.Error("Failed to cleanly stop listener: %s", err.Error())
		}
	}

	os.Exit(exitCode)
}

// expandPath will check if the given path begins with a "~" symbol, and if so,
// will expand it to become the user's home directory.
func expandPath(path string) (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}

	if path == "~" {
		return usr.HomeDir, nil
	} else if strings.HasPrefix(path, "~/") {
		return filepath.Join(usr.HomeDir, path[2:]), nil
	}

	return path, nil
}

func newTestHarnessSocketVal(logger log.Logger, cfg TestHarnessConfig) (*privval.SocketVal, error) {
	proto, addr := cmn.ProtocolAndAddress(cfg.BindAddr)
	if proto == "unix" {
		// make sure the socket doesn't exist - if so, try to delete it
		if cmn.FileExists(addr) {
			if err := os.Remove(addr); err != nil {
				logger.Error("Failed to remove existing Unix domain socket", "addr", addr)
				return nil, err
			}
		}
	}
	ln, err := net.Listen(proto, addr)
	logger.Info("Listening at", "proto", proto, "addr", addr)
	if err != nil {
		return nil, err
	}
	var svln net.Listener
	if proto == "unix" {
		unixLn := privval.NewUnixListener(ln)
		privval.UnixListenerAcceptDeadline(cfg.AcceptDeadline)(unixLn)
		privval.UnixListenerConnDeadline(cfg.ConnDeadline)(unixLn)
		svln = unixLn
	} else {
		tcpLn := privval.NewTCPListener(ln, cfg.SecretConnKey)
		privval.TCPListenerAcceptDeadline(cfg.AcceptDeadline)(tcpLn)
		privval.TCPListenerConnDeadline(cfg.ConnDeadline)(tcpLn)
		logger.Info("Resolved TCP address for listener", "addr", tcpLn.Addr())
		svln = tcpLn
	}
	return privval.NewSocketVal(logger, svln), nil
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
	case ErrFailedToExpandPath:
		msg = "Failed to expand path"
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
