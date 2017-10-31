package types

// Interface assertions
var _ TxEventPublisher = (*TxEventBuffer)(nil)

// TxEventBuffer is a buffer of events, which uses a slice to temporarily store
// events.
type TxEventBuffer struct {
	next     TxEventPublisher
	capacity int
	events   []EventDataTx
}

// NewTxEventBuffer accepts a TxEventPublisher and returns a new buffer with the given
// capacity.
func NewTxEventBuffer(next TxEventPublisher, capacity int) *TxEventBuffer {
	return &TxEventBuffer{
		next:     next,
		capacity: capacity,
		events:   make([]EventDataTx, 0, capacity),
	}
}

// Len returns the number of events cached.
func (b TxEventBuffer) Len() int {
	return len(b.events)
}

// PublishEventTx buffers an event to be fired upon finality.
func (b *TxEventBuffer) PublishEventTx(e EventDataTx) error {
	b.events = append(b.events, e)
	return nil
}

// Flush publishes events by running next.PublishWithTags on all cached events.
// Blocks. Clears cached events.
func (b *TxEventBuffer) Flush() error {
	for _, e := range b.events {
		err := b.next.PublishEventTx(e)
		if err != nil {
			return err
		}
	}
	b.events = make([]EventDataTx, 0, b.capacity)
	return nil
}
