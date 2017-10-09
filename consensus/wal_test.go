package consensus

import (
	"bytes"
	"testing"
	"time"

	"github.com/tendermint/tendermint/consensus/types"

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
