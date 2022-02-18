package consensus

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	mrand "math/rand"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	abciclient "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/internal/eventbus"
	"github.com/tendermint/tendermint/internal/proxy"
	sm "github.com/tendermint/tendermint/internal/state"
	"github.com/tendermint/tendermint/internal/store"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/privval"
	"github.com/tendermint/tendermint/types"
)

// WALGenerateNBlocks generates a consensus WAL. It does this by spinning up a
// stripped down version of node (proxy app, event bus, consensus state) with a
// persistent kvstore application and special consensus wal instance
// (byteBufferWAL) and waits until numBlocks are created.
// If the node fails to produce given numBlocks, it returns an error.
func WALGenerateNBlocks(ctx context.Context, t *testing.T, logger log.Logger, wr io.Writer, numBlocks int) (err error) {
	cfg := getConfig(t)

	app := kvstore.NewPersistentKVStoreApplication(logger, filepath.Join(cfg.DBDir(), "wal_generator"))
	t.Cleanup(func() { require.NoError(t, app.Close()) })

	logger.Info("generating WAL (last height msg excluded)", "numBlocks", numBlocks)

	// COPY PASTE FROM node.go WITH A FEW MODIFICATIONS
	// NOTE: we can't import node package because of circular dependency.
	// NOTE: we don't do handshake so need to set state.Version.Consensus.App directly.
	privValidatorKeyFile := cfg.PrivValidator.KeyFile()
	privValidatorStateFile := cfg.PrivValidator.StateFile()
	privValidator, err := privval.LoadOrGenFilePV(privValidatorKeyFile, privValidatorStateFile)
	if err != nil {
		return err
	}
	genDoc, err := types.GenesisDocFromFile(cfg.GenesisFile())
	if err != nil {
		return fmt.Errorf("failed to read genesis file: %w", err)
	}
	blockStoreDB := dbm.NewMemDB()
	stateDB := blockStoreDB
	stateStore := sm.NewStore(stateDB)
	state, err := sm.MakeGenesisState(genDoc)
	if err != nil {
		return fmt.Errorf("failed to make genesis state: %w", err)
	}
	state.Version.Consensus.App = kvstore.ProtocolVersion
	if err = stateStore.Save(state); err != nil {
		t.Error(err)
	}

	blockStore := store.NewBlockStore(blockStoreDB)

	proxyApp := proxy.NewAppConns(abciclient.NewLocalCreator(app), logger.With("module", "proxy"), proxy.NopMetrics())
	if err := proxyApp.Start(ctx); err != nil {
		return fmt.Errorf("failed to start proxy app connections: %w", err)
	}

	eventBus := eventbus.NewDefault(logger.With("module", "events"))
	if err := eventBus.Start(ctx); err != nil {
		return fmt.Errorf("failed to start event bus: %w", err)
	}

	mempool := emptyMempool{}
	evpool := sm.EmptyEvidencePool{}
	blockExec := sm.NewBlockExecutor(stateStore, log.TestingLogger(), proxyApp.Consensus(), mempool, evpool, blockStore)
	consensusState := NewState(ctx, logger, cfg.Consensus, state.Copy(), blockExec, blockStore, mempool, evpool)
	consensusState.SetEventBus(eventBus)
	if privValidator != nil && privValidator != (*privval.FilePV)(nil) {
		consensusState.SetPrivValidator(ctx, privValidator)
	}
	// END OF COPY PASTE

	// set consensus wal to buffered WAL, which will write all incoming msgs to buffer
	numBlocksWritten := make(chan struct{})
	wal := newByteBufferWAL(logger, NewWALEncoder(wr), int64(numBlocks), numBlocksWritten)
	// see wal.go#103
	if err := wal.Write(EndHeightMessage{0}); err != nil {
		t.Error(err)
	}

	consensusState.wal = wal

	if err := consensusState.Start(ctx); err != nil {
		return fmt.Errorf("failed to start consensus state: %w", err)
	}

	select {
	case <-numBlocksWritten:
		consensusState.Stop()
		return nil
	case <-time.After(1 * time.Minute):
		consensusState.Stop()
		return fmt.Errorf("waited too long for tendermint to produce %d blocks (grep logs for `wal_generator`)", numBlocks)
	}
}

// WALWithNBlocks returns a WAL content with numBlocks.
func WALWithNBlocks(ctx context.Context, t *testing.T, logger log.Logger, numBlocks int) (data []byte, err error) {
	var b bytes.Buffer
	wr := bufio.NewWriter(&b)

	if err := WALGenerateNBlocks(ctx, t, logger, wr, numBlocks); err != nil {
		return []byte{}, err
	}

	wr.Flush()
	return b.Bytes(), nil
}

func randPort() int {
	// returns between base and base + spread
	base, spread := 20000, 20000
	// nolint:gosec // G404: Use of weak random number generator
	return base + mrand.Intn(spread)
}

// makeAddrs constructs local TCP addresses for node services.
// It uses consecutive ports from a random starting point, so that concurrent
// instances are less likely to collide.
func makeAddrs() (p2pAddr, rpcAddr string) {
	const addrTemplate = "tcp://127.0.0.1:%d"
	start := randPort()
	return fmt.Sprintf(addrTemplate, start), fmt.Sprintf(addrTemplate, start+1)
}

// getConfig returns a config for test cases
func getConfig(t *testing.T) *config.Config {
	c, err := config.ResetTestRoot(t.TempDir(), t.Name())
	require.NoError(t, err)

	p2pAddr, rpcAddr := makeAddrs()
	c.P2P.ListenAddress = p2pAddr
	c.RPC.ListenAddress = rpcAddr
	return c
}

// byteBufferWAL is a WAL which writes all msgs to a byte buffer. Writing stops
// when the heightToStop is reached. Client will be notified via
// signalWhenStopsTo channel.
type byteBufferWAL struct {
	enc               *WALEncoder
	stopped           bool
	heightToStop      int64
	signalWhenStopsTo chan<- struct{}

	logger log.Logger
}

// needed for determinism
var fixedTime, _ = time.Parse(time.RFC3339, "2017-01-02T15:04:05Z")

func newByteBufferWAL(logger log.Logger, enc *WALEncoder, nBlocks int64, signalStop chan<- struct{}) *byteBufferWAL {
	return &byteBufferWAL{
		enc:               enc,
		heightToStop:      nBlocks,
		signalWhenStopsTo: signalStop,
		logger:            logger,
	}
}

// Save writes message to the internal buffer except when heightToStop is
// reached, in which case it will signal the caller via signalWhenStopsTo and
// skip writing.
func (w *byteBufferWAL) Write(m WALMessage) error {
	if w.stopped {
		w.logger.Debug("WAL already stopped. Not writing message", "msg", m)
		return nil
	}

	if endMsg, ok := m.(EndHeightMessage); ok {
		w.logger.Debug("WAL write end height message", "height", endMsg.Height, "stopHeight", w.heightToStop)
		if endMsg.Height == w.heightToStop {
			w.logger.Debug("Stopping WAL at height", "height", endMsg.Height)
			w.signalWhenStopsTo <- struct{}{}
			w.stopped = true
			return nil
		}
	}

	w.logger.Debug("WAL Write Message", "msg", m)
	err := w.enc.Encode(&TimedWALMessage{fixedTime, m})
	if err != nil {
		panic(fmt.Sprintf("failed to encode the msg %v", m))
	}

	return nil
}

func (w *byteBufferWAL) WriteSync(m WALMessage) error {
	return w.Write(m)
}

func (w *byteBufferWAL) FlushAndSync() error { return nil }

func (w *byteBufferWAL) SearchForEndHeight(
	height int64,
	options *WALSearchOptions) (rd io.ReadCloser, found bool, err error) {
	return nil, false, nil
}

func (w *byteBufferWAL) Start(context.Context) error { return nil }
func (w *byteBufferWAL) Stop()                       {}
func (w *byteBufferWAL) Wait()                       {}
