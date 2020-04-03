package blockchain

import (
	"errors"
	"fmt"

	amino "github.com/tendermint/go-amino"

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

// RegisterBlockchainMessages registers the fast sync messages for amino encoding.
func RegisterBlockchainMessages(cdc *amino.Codec) {
	Cdc.RegisterInterface((*Message)(nil), nil)
	Cdc.RegisterConcrete(&BlockRequestMessage{}, "tendermint/blockchain/BlockRequest", nil)
	Cdc.RegisterConcrete(&BlockResponseMessage{}, "tendermint/blockchain/BlockResponse", nil)
	Cdc.RegisterConcrete(&NoBlockResponseMessage{}, "tendermint/blockchain/NoBlockResponse", nil)
	Cdc.RegisterConcrete(&StatusResponseMessage{}, "tendermint/blockchain/StatusResponse", nil)
	Cdc.RegisterConcrete(&StatusRequestMessage{}, "tendermint/blockchain/StatusRequest", nil)
}

func decodeMsg(bz []byte) (msg Message, err error) {
	if len(bz) > maxMsgSize {
		return msg, fmt.Errorf("msg exceeds max size (%d > %d)", len(bz), maxMsgSize)
	}
	err = Cdc.UnmarshalBinaryBare(bz, &msg)
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
