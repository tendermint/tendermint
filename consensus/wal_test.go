package consensus

import (
	"bytes"
	"crypto/rand"
	// "sync"
	"testing"
	"time"

	"github.com/tendermint/tendermint/consensus/types"
	tmtypes "github.com/tendermint/tendermint/types"
	cmn "github.com/tendermint/tendermint/libs/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWALEncoderDecoder(t *testing.T) {
	now := time.Now()
	msgs := []TimedWALMessage{
		TimedWALMessage{Time: now, Msg: EndHeightMessage{0}},
		TimedWALMessage{Time: now, Msg: timeoutInfo{Duration: time.Second, Height: 1, Round: 1, Step: types.RoundStepPropose}},
	}

	b := new(bytes.Buffer)

	for _, msg := range msgs {
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

func TestWALSearchForEndHeight(t *testing.T) {
	walBody, err := WALWithNBlocks(6)
	if err != nil {
		t.Fatal(err)
	}
	walFile := tempWALWithData(walBody)

	wal, err := NewWAL(walFile)
	if err != nil {
		t.Fatal(err)
	}

	h := int64(3)
	gr, found, err := wal.SearchForEndHeight(h, &WALSearchOptions{})
	assert.NoError(t, err, cmn.Fmt("expected not to err on height %d", h))
	assert.True(t, found, cmn.Fmt("expected to find end height for %d", h))
	assert.NotNil(t, gr, "expected group not to be nil")
	defer gr.Close()

	dec := NewWALDecoder(gr)
	msg, err := dec.Decode()
	assert.NoError(t, err, "expected to decode a message")
	rs, ok := msg.Msg.(tmtypes.EventDataRoundState)
	assert.True(t, ok, "expected message of type EventDataRoundState")
	assert.Equal(t, rs.Height, h+1, cmn.Fmt("wrong height"))
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
	enc.Encode(&TimedWALMessage{Msg: data, Time: time.Now().Round(time.Second)})

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
