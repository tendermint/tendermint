package consensus

import (
	"bytes"
	"crypto/rand"
	"io/ioutil"
	"os"
	"path/filepath"

	// "sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/consensus/types"
	"github.com/tendermint/tendermint/crypto/merkle"
	"github.com/tendermint/tendermint/libs/autofile"
	"github.com/tendermint/tendermint/libs/log"
	tmtypes "github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
)

const (
	walTestFlushInterval = time.Duration(100) * time.Millisecond
)

func TestWALTruncate(t *testing.T) {
	walDir, err := ioutil.TempDir("", "wal")
	require.NoError(t, err)
	defer os.RemoveAll(walDir)

	walFile := filepath.Join(walDir, "wal")

	// this magic number 4K can truncate the content when RotateFile.
	// defaultHeadSizeLimit(10M) is hard to simulate.
	// this magic number 1 * time.Millisecond make RotateFile check frequently.
	// defaultGroupCheckDuration(5s) is hard to simulate.
	wal, err := NewWAL(walFile,
		autofile.GroupHeadSizeLimit(4096),
		autofile.GroupCheckDuration(1*time.Millisecond),
	)
	require.NoError(t, err)
	wal.SetLogger(log.TestingLogger())
	err = wal.Start()
	require.NoError(t, err)
	defer func() {
		wal.Stop()
		// wait for the wal to finish shutting down so we
		// can safely remove the directory
		wal.Wait()
	}()

	// 60 block's size nearly 70K, greater than group's headBuf size(4096 * 10),
	// when headBuf is full, truncate content will Flush to the file. at this
	// time, RotateFile is called, truncate content exist in each file.
	err = WALGenerateNBlocks(t, wal.Group(), 60)
	require.NoError(t, err)

	time.Sleep(1 * time.Millisecond) //wait groupCheckDuration, make sure RotateFile run

	wal.FlushAndSync()

	h := int64(50)
	gr, found, err := wal.SearchForEndHeight(h, &WALSearchOptions{})
	assert.NoError(t, err, "expected not to err on height %d", h)
	assert.True(t, found, "expected to find end height for %d", h)
	assert.NotNil(t, gr)
	defer gr.Close()

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
	walDir, err := ioutil.TempDir("", "wal")
	require.NoError(t, err)
	defer os.RemoveAll(walDir)
	walFile := filepath.Join(walDir, "wal")

	wal, err := NewWAL(walFile)
	require.NoError(t, err)
	err = wal.Start()
	require.NoError(t, err)
	defer func() {
		wal.Stop()
		// wait for the wal to finish shutting down so we
		// can safely remove the directory
		wal.Wait()
	}()

	// 1) Write returns an error if msg is too big
	msg := &BlockPartMessage{
		Height: 1,
		Round:  1,
		Part: &tmtypes.Part{
			Index: 1,
			Bytes: make([]byte, 1),
			Proof: merkle.SimpleProof{
				Total:    1,
				Index:    1,
				LeafHash: make([]byte, maxMsgSizeBytes-30),
			},
		},
	}
	err = wal.Write(msg)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "msg is too big")
	}
}

func TestWALSearchForEndHeight(t *testing.T) {
	walBody, err := WALWithNBlocks(t, 6)
	if err != nil {
		t.Fatal(err)
	}
	walFile := tempWALWithData(walBody)

	wal, err := NewWAL(walFile)
	require.NoError(t, err)
	wal.SetLogger(log.TestingLogger())

	h := int64(3)
	gr, found, err := wal.SearchForEndHeight(h, &WALSearchOptions{})
	assert.NoError(t, err, "expected not to err on height %d", h)
	assert.True(t, found, "expected to find end height for %d", h)
	assert.NotNil(t, gr)
	defer gr.Close()

	dec := NewWALDecoder(gr)
	msg, err := dec.Decode()
	assert.NoError(t, err, "expected to decode a message")
	rs, ok := msg.Msg.(tmtypes.EventDataRoundState)
	assert.True(t, ok, "expected message of type EventDataRoundState")
	assert.Equal(t, rs.Height, h+1, "wrong height")
}

func TestWALPeriodicSync(t *testing.T) {
	walDir, err := ioutil.TempDir("", "wal")
	require.NoError(t, err)
	defer os.RemoveAll(walDir)

	walFile := filepath.Join(walDir, "wal")
	wal, err := NewWAL(walFile, autofile.GroupCheckDuration(1*time.Millisecond))
	require.NoError(t, err)

	wal.SetFlushInterval(walTestFlushInterval)
	wal.SetLogger(log.TestingLogger())

	// Generate some data
	err = WALGenerateNBlocks(t, wal.Group(), 5)
	require.NoError(t, err)

	// We should have data in the buffer now
	assert.NotZero(t, wal.Group().Buffered())

	require.NoError(t, wal.Start())
	defer func() {
		wal.Stop()
		wal.Wait()
	}()

	time.Sleep(walTestFlushInterval + (10 * time.Millisecond))

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
}

/*
var initOnce sync.Once

func registerInterfacesOnce() {
	initOnce.Do(func() {
		var _ = wire.RegisterInterface(
			struct{ WALMessage }{},
			wire.ConcreteType{[]byte{}, 0x10},
		)
	})
}
*/

func nBytes(n int) []byte {
	buf := make([]byte, n)
	n, _ = rand.Read(buf)
	return buf[:n]
}

func benchmarkWalDecode(b *testing.B, n int) {
	// registerInterfacesOnce()

	buf := new(bytes.Buffer)
	enc := NewWALEncoder(buf)

	data := nBytes(n)
	enc.Encode(&TimedWALMessage{Msg: data, Time: time.Now().Round(time.Second).UTC()})

	encoded := buf.Bytes()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		buf.Write(encoded)
		dec := NewWALDecoder(buf)
		if _, err := dec.Decode(); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportAllocs()
}

func BenchmarkWalDecode512B(b *testing.B) {
	benchmarkWalDecode(b, 512)
}

func BenchmarkWalDecode10KB(b *testing.B) {
	benchmarkWalDecode(b, 10*1024)
}
func BenchmarkWalDecode100KB(b *testing.B) {
	benchmarkWalDecode(b, 100*1024)
}
func BenchmarkWalDecode1MB(b *testing.B) {
	benchmarkWalDecode(b, 1024*1024)
}
func BenchmarkWalDecode10MB(b *testing.B) {
	benchmarkWalDecode(b, 10*1024*1024)
}
func BenchmarkWalDecode100MB(b *testing.B) {
	benchmarkWalDecode(b, 100*1024*1024)
}
func BenchmarkWalDecode1GB(b *testing.B) {
	benchmarkWalDecode(b, 1024*1024*1024)
}
