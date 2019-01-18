package privval

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/tendermint/tendermint/crypto/ed25519"
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
)

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
	sc      *SocketVal
	fpv     *FilePV
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
	fpv := LoadFilePV(keyFile, stateFile)

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
		th.TestPublicKey()
		th.TestSignProposal()
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

func (th *TestHarness) TestPublicKey() {
	th.logger.Info("TEST: Public key of remote signer")
	th.logger.Info("Local", "pubKey", th.fpv.GetPubKey())
	th.logger.Info("Remote", "pubKey", th.sc.GetPubKey())
	if th.fpv.GetPubKey() != th.sc.GetPubKey() {
		th.logger.Error("FAILED: Local and remote public keys do not match")
		th.Shutdown(newTestHarnessError(ErrTestPublicKeyFailed, nil, ""))
	}
}

func (th *TestHarness) TestSignProposal() {
	th.logger.Info("TEST: Signing of proposals")
	prop := &types.Proposal{Timestamp: time.Now()}
	if err := th.sc.SignProposal(th.chainID, prop); err != nil {
		th.logger.Error("FAILED: Signing of proposal", "err", err)
		th.Shutdown(newTestHarnessError(ErrTestSignProposalFailed, err, ""))
	}
	th.logger.Info("Signed proposal", "prop", prop)
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

func newTestHarnessSocketVal(logger log.Logger, cfg TestHarnessConfig) (*SocketVal, error) {
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
		unixLn := NewUnixListener(ln)
		UnixListenerAcceptDeadline(cfg.AcceptDeadline)(unixLn)
		UnixListenerConnDeadline(cfg.ConnDeadline)(unixLn)
		svln = unixLn
	} else {
		tcpLn := NewTCPListener(ln, cfg.SecretConnKey)
		TCPListenerAcceptDeadline(cfg.AcceptDeadline)(tcpLn)
		TCPListenerConnDeadline(cfg.ConnDeadline)(tcpLn)
		logger.Info("Resolved TCP address for listener", "addr", tcpLn.Addr())
		svln = tcpLn
	}
	return NewSocketVal(logger, svln), nil
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
