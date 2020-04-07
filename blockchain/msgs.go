package blockchain

import (
	"errors"
	"fmt"

	bcproto "github.com/tendermint/tendermint/proto/blockchain"
	"github.com/tendermint/tendermint/types"
)

const (
	// NOTE: keep up to date with bcBlockResponseMessage
	BlockResponseMessagePrefixSize   = 4
	BlockResponseMessageFieldKeySize = 1
	MaxMsgSize                       = types.MaxBlockSizeBytes +
		BlockResponseMessagePrefixSize +
		BlockResponseMessageFieldKeySize
)

// BlockchainMessage is a generic message for this reactor.
type Message interface {
	ValidateBasic() error
}

func DecodeMsg(bz []byte) (msg Message, err error) {
	if len(bz) > MaxMsgSize {
		return msg, fmt.Errorf("msg exceeds max size (%d > %d)", len(bz), MaxMsgSize)
	}
	bm := bcproto.Message{}
	bm.Unmarshal(bz)
	msg, err = MsgFromProto(bm)
	return
}

//-------------------------------------

type BlockRequestMessage struct {
	Height int64
}

// ValidateBasic performs basic validation.
func (m *BlockRequestMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("negative Height")
	}
	return nil
}

func (m *BlockRequestMessage) String() string {
	return fmt.Sprintf("[BlockRequestMessage %v]", m.Height)
}

type NoBlockResponseMessage struct {
	Height int64
}

// ValidateBasic performs basic validation.
func (m *NoBlockResponseMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("negative Height")
	}
	return nil
}

func (m *NoBlockResponseMessage) String() string {
	return fmt.Sprintf("[NoBlockResponseMessage %d]", m.Height)
}

//-------------------------------------

type BlockResponseMessage struct {
	Block *types.Block
}

// ValidateBasic performs basic validation.
func (m *BlockResponseMessage) ValidateBasic() error {
	return m.Block.ValidateBasic()
}

func (m *BlockResponseMessage) String() string {
	return fmt.Sprintf("[BlockResponseMessage %v]", m.Block.Height)
}

//-------------------------------------

type StatusRequestMessage struct {
	Base   int64
	Height int64
}

// ValidateBasic performs basic validation.
func (m *StatusRequestMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("negative Height")
	}
	return nil
}

func (m *StatusRequestMessage) String() string {
	return fmt.Sprintf("[StatusRequestMessage %v]", m.Height)
}

//-------------------------------------

type StatusResponseMessage struct {
	Height int64
	Base   int64
}

// ValidateBasic performs basic validation.
func (m *StatusResponseMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("negative Height")
	}
	return nil
}

func (m *StatusResponseMessage) String() string {
	return fmt.Sprintf("[StatusResponseMessage %v]", m.Height)
}
