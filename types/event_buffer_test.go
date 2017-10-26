package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type eventBusMock struct{}

func (eventBusMock) PublishEventTx(e EventDataTx) error {
	return nil
}

func TestEventBuffer(t *testing.T) {
	b := NewTxEventBuffer(eventBusMock{}, 1)
	b.PublishEventTx(EventDataTx{})
	assert.Equal(t, 1, b.Len())
	b.Flush()
	assert.Equal(t, 0, b.Len())
}
