package consensus

import (
	"bytes"
	"path"
	"testing"
	"time"

	"github.com/tendermint/tendermint/consensus/types"
	tmtypes "github.com/tendermint/tendermint/types"
	cmn "github.com/tendermint/tmlibs/common"

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

		assert.Equal(t, msg.Time.Truncate(time.Millisecond), decoded.Time)
		assert.Equal(t, msg.Msg, decoded.Msg)
	}
}

func TestSearchForEndHeight(t *testing.T) {
	wal, err := NewWAL(path.Join(data_dir, "many_blocks.cswal"), false)
	if err != nil {
		t.Fatal(err)
	}

	h := 3
	gr, found, err := wal.SearchForEndHeight(uint64(h))
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
