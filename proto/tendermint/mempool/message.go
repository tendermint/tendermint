package mempool

import (
	"fmt"

	"github.com/cosmos/gogoproto/proto"
	"github.com/tendermint/tendermint/p2p"
)

var _ p2p.Wrapper = &Txs{}

// Wrap implements the p2p Wrapper interface and wraps a mempool message.
func (m *Txs) Wrap() (proto.Message, error) {
	return &Message{
		Sum: &Message_Txs{Txs: m},
	}, nil
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
