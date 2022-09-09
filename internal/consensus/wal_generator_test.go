package consensus

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	mrand "math/rand"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/log"
)

// WALGenerateNBlocks generates a consensus WAL. It does this by
// spinning up a stripped down version of node (proxy app, event bus,
// consensus state) with a kvstore application and special consensus
// wal instance (byteBufferWAL) and waits until numBlocks are created.
// If the node fails to produce given numBlocks, it fails the test.
func WALGenerateNBlocks(ctx context.Context, t *testing.T, logger log.Logger, wr io.Writer, node *fakeNode, numBlocks int) {
	t.Helper()

	logger.Info("generating WAL (last height msg excluded)", "numBlocks", numBlocks)

	// set consensus wal to buffered WAL, which will write all incoming msgs to buffer
	numBlocksWritten := make(chan struct{})
	wal := newByteBufferWAL(logger, NewWALEncoder(wr), int64(numBlocks), numBlocksWritten)
	// see wal.go#103
	if err := wal.Write(EndHeightMessage{0}); err != nil {
		t.Fatal(err)
	}

	node.csState.wal = wal

	node.start(ctx, t)
	defer node.stop()

	timer := time.NewTimer(time.Minute)
	defer timer.Stop()

	select {
	case <-numBlocksWritten:
	case <-timer.C:
		t.Fatal(fmt.Errorf("waited too long for tendermint to produce %d blocks (grep logs for `wal_generator`)", numBlocks))
	}
}

// WALWithNBlocks returns a WAL content with numBlocks.
func WALWithNBlocks(ctx context.Context, t *testing.T, logger log.Logger, node *fakeNode, numBlocks int) (data []byte, err error) {
	var b bytes.Buffer
	wr := bufio.NewWriter(&b)

	WALGenerateNBlocks(ctx, t, logger, wr, node, numBlocks)

	wr.Flush()
	return b.Bytes(), nil
}

func randPort() int {
	// returns between base and base + spread
	base, spread := 20000, 20000
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
	testName := strings.ReplaceAll(t.Name(), "/", "_")
	c, err := config.ResetTestRoot(t.TempDir(), testName)
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

func newByteBufferWAL(
	logger log.Logger,
	enc *WALEncoder,
	nBlocks int64,
	signalStop chan<- struct{},
) *byteBufferWAL {
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
		w.logger.Debug(
			"WAL write end height message",
			"height",
			endMsg.Height,
			"stopHeight",
			w.heightToStop,
		)
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
