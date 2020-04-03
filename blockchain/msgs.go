package blockchain

import (
	"errors"
	"fmt"

	"github.com/gogo/protobuf/proto"

	bcproto "github.com/tendermint/tendermint/proto/blockchain"
	"github.com/tendermint/tendermint/types"
)

const (
	// NOTE: keep up to date with bcBlockResponseMessage
	bcBlockResponseMessagePrefixSize   = 4
	bcBlockResponseMessageFieldKeySize = 1
	maxMsgSize                         = types.MaxBlockSizeBytes +
		bcBlockResponseMessagePrefixSize +
		bcBlockResponseMessageFieldKeySize
)

// BlockchainMessage is a generic message for this reactor.
type Message interface {
	ValidateBasic() error
}

func DecodeMsg(bz []byte) (msg Message, err error) {
	if len(bz) > maxMsgSize {
		return msg, fmt.Errorf("msg exceeds max size (%d > %d)", len(bz), maxMsgSize)
	}
	bm := bcproto.Message{}
	err = proto.Unmarshal(bz, &bm)
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
