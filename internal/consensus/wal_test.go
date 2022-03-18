package consensus

import (
	"bytes"
	"context"
	"os"
	"path/filepath"

	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto/merkle"
	"github.com/tendermint/tendermint/internal/consensus/types"
	"github.com/tendermint/tendermint/internal/libs/autofile"
	"github.com/tendermint/tendermint/libs/log"
	tmtime "github.com/tendermint/tendermint/libs/time"
	tmtypes "github.com/tendermint/tendermint/types"
)

const walTestFlushInterval = 100 * time.Millisecond

func TestWALTruncate(t *testing.T) {
	walDir := t.TempDir()
	walFile := filepath.Join(walDir, "wal")
	logger := log.NewNopLogger()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// this magic number 4K can truncate the content when RotateFile.
	// defaultHeadSizeLimit(10M) is hard to simulate.
	// this magic number 1 * time.Millisecond make RotateFile check frequently.
	// defaultGroupCheckDuration(5s) is hard to simulate.
	wal, err := NewWAL(ctx, logger, walFile,
		autofile.GroupHeadSizeLimit(4096),
		autofile.GroupCheckDuration(1*time.Millisecond),
	)
	require.NoError(t, err)
	err = wal.Start(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { wal.Stop(); wal.Group().Stop(); wal.Group().Wait(); wal.Wait() })

	// 60 block's size nearly 70K, greater than group's headBuf size(4096 * 10),
	// when headBuf is full, truncate content will Flush to the file. at this
	// time, RotateFile is called, truncate content exist in each file.
	WALGenerateNBlocks(ctx, t, logger, wal.Group(), 60)

	// put the leakcheck here so it runs after other cleanup
	// functions.
	t.Cleanup(leaktest.CheckTimeout(t, 500*time.Millisecond))

	time.Sleep(1 * time.Millisecond) // wait groupCheckDuration, make sure RotateFile run

	if err := wal.FlushAndSync(); err != nil {
		t.Error(err)
	}

	h := int64(50)
	gr, found, err := wal.SearchForEndHeight(h, &WALSearchOptions{})
	assert.NoError(t, err, "expected not to err on height %d", h)
	assert.True(t, found, "expected to find end height for %d", h)
	assert.NotNil(t, gr)
	t.Cleanup(func() { _ = gr.Close() })

	dec := NewWALDecoder(gr)
	msg, err := dec.Decode()
	assert.NoError(t, err, "expected to decode a message")
	rs, ok := msg.Msg.(tmtypes.EventDataRoundState)
	assert.True(t, ok, "expected message of type EventDataRoundState")
	assert.Equal(t, rs.Height, h+1, "wrong height")
}

func TestWALEncoderDecoder(t *testing.T) {
	now := tmtime.Now()
	msgs := []TimedWALMessage{
		{Time: now, Msg: EndHeightMessage{0}},
		{Time: now, Msg: timeoutInfo{Duration: time.Second, Height: 1, Round: 1, Step: types.RoundStepPropose}},
		{Time: now, Msg: tmtypes.EventDataRoundState{Height: 1, Round: 1, Step: ""}},
	}

	b := new(bytes.Buffer)

	for _, msg := range msgs {
		msg := msg

		b.Reset()

		enc := NewWALEncoder(b)
		err := enc.Encode(&msg)
		require.NoError(t, err)

		dec := NewWALDecoder(b)
		decoded, err := dec.Decode()
		require.NoError(t, err)
		assert.Equal(t, msg.Time.UTC(), decoded.Time)
		assert.Equal(t, msg.Msg, decoded.Msg)
	}
}

func TestWALWrite(t *testing.T) {
	walDir := t.TempDir()
	walFile := filepath.Join(walDir, "wal")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wal, err := NewWAL(ctx, log.NewNopLogger(), walFile)
	require.NoError(t, err)
	err = wal.Start(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { wal.Stop(); wal.Group().Stop(); wal.Group().Wait(); wal.Wait() })

	// 1) Write returns an error if msg is too big
	msg := &BlockPartMessage{
		Height: 1,
		Round:  1,
		Part: &tmtypes.Part{
			Index: 1,
			Bytes: make([]byte, 1),
			Proof: merkle.Proof{
				Total:    1,
				Index:    1,
				LeafHash: make([]byte, maxMsgSizeBytes-30),
			},
		},
	}

	err = wal.Write(msgInfo{
		Msg: msg,
	})
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "msg is too big")
	}
}

func TestWALSearchForEndHeight(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()

	walBody, err := WALWithNBlocks(ctx, t, logger, 6)
	if err != nil {
		t.Fatal(err)
	}
	walFile := tempWALWithData(t, walBody)

	wal, err := NewWAL(ctx, logger, walFile)
	require.NoError(t, err)

	h := int64(3)
	gr, found, err := wal.SearchForEndHeight(h, &WALSearchOptions{})
	assert.NoError(t, err, "expected not to err on height %d", h)
	assert.True(t, found, "expected to find end height for %d", h)
	assert.NotNil(t, gr)
	t.Cleanup(func() { _ = gr.Close() })

	dec := NewWALDecoder(gr)
	msg, err := dec.Decode()
	assert.NoError(t, err, "expected to decode a message")
	rs, ok := msg.Msg.(tmtypes.EventDataRoundState)
	assert.True(t, ok, "expected message of type EventDataRoundState")
	assert.Equal(t, rs.Height, h+1, "wrong height")

	t.Cleanup(leaktest.Check(t))
}

func TestWALPeriodicSync(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	walDir := t.TempDir()
	walFile := filepath.Join(walDir, "wal")
	defer os.RemoveAll(walFile)

	wal, err := NewWAL(ctx, log.NewNopLogger(), walFile, autofile.GroupCheckDuration(250*time.Millisecond))
	require.NoError(t, err)

	wal.SetFlushInterval(walTestFlushInterval)
	logger := log.NewNopLogger()

	// Generate some data
	WALGenerateNBlocks(ctx, t, logger, wal.Group(), 5)

	// We should have data in the buffer now
	assert.NotZero(t, wal.Group().Buffered())

	require.NoError(t, wal.Start(ctx))
	t.Cleanup(func() { wal.Stop(); wal.Group().Stop(); wal.Group().Wait(); wal.Wait() })

	time.Sleep(walTestFlushInterval + (20 * time.Millisecond))

	// The data should have been flushed by the periodic sync
	assert.Zero(t, wal.Group().Buffered())

	h := int64(4)
	gr, found, err := wal.SearchForEndHeight(h, &WALSearchOptions{})
	assert.NoError(t, err, "expected not to err on height %d", h)
	assert.True(t, found, "expected to find end height for %d", h)
	assert.NotNil(t, gr)
	if gr != nil {
		gr.Close()
	}

	t.Cleanup(leaktest.Check(t))
}
