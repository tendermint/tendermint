package mempool

import (
	"fmt"

	"github.com/gogo/protobuf/proto"
)

// Wrap implements the p2p Wrapper interface and wraps a mempool message.
func (m *Message) Wrap(pb proto.Message) error {
	switch msg := pb.(type) {
	case *Txs:
		m.Sum = &Message_Txs{Txs: msg}

	default:
		return fmt.Errorf("unknown message: %T", msg)
	}

	return nil
}

// Unwrap implements the p2p Wrapper interface and unwraps a wrapped mempool
// message.
func (m *Message) Unwrap() (proto.Message, error) {
	switch msg := m.Sum.(type) {
	case *Message_Txs:
		return m.GetTxs(), nil

	default:
		return nil, fmt.Errorf("unknown message: %T", msg)
	}
}
