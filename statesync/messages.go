package statesync

import (
	"errors"
	"fmt"

	amino "github.com/tendermint/go-amino"

	"github.com/tendermint/tendermint/types"
)

const (
	// snapshotMsgSize is the maximum size of a snapshotResponseMessage
	snapshotMsgSize = int(4e6)
	// chunkMsgSize is the maximum size of a chunkResponseMessage
	chunkMsgSize = int(16e6)
	// maxMsgSize is the maximum size of any message
	maxMsgSize = chunkMsgSize
)

var cdc = amino.NewCodec()

func init() {
	cdc.RegisterInterface((*Message)(nil), nil)
	cdc.RegisterConcrete(&snapshotsRequestMessage{}, "tendermint/SnapshotsRequestMessage", nil)
	cdc.RegisterConcrete(&snapshotsResponseMessage{}, "tendermint/SnapshotsResponseMessage", nil)
	cdc.RegisterConcrete(&chunkRequestMessage{}, "tendermint/ChunkRequestMessage", nil)
	cdc.RegisterConcrete(&chunkResponseMessage{}, "tendermint/ChunkResponseMessage", nil)
	types.RegisterBlockAmino(cdc)
}

// decodeMsg decodes a message.
func decodeMsg(bz []byte) (Message, error) {
	if len(bz) > maxMsgSize {
		return nil, fmt.Errorf("msg exceeds max size (%d > %d)", len(bz), maxMsgSize)
	}
	var msg Message
	err := cdc.UnmarshalBinaryBare(bz, &msg)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

// Message is a message sent and received by the reactor.
type Message interface {
	ValidateBasic() error
}

// snapshotsRequestMessage requests recent snapshots from a peer.
type snapshotsRequestMessage struct{}

// ValidateBasic implements Message.
func (m *snapshotsRequestMessage) ValidateBasic() error {
	if m == nil {
		return errors.New("nil message")
	}
	return nil
}

// SnapshotResponseMessage contains information about a single snapshot.
type snapshotsResponseMessage struct {
	Height   uint64
	Format   uint32
	Chunks   uint32
	Hash     []byte
	Metadata []byte
}

// ValidateBasic implements Message.
func (m *snapshotsResponseMessage) ValidateBasic() error {
	if m == nil {
		return errors.New("nil message")
	}
	if m.Height == 0 {
		return errors.New("height cannot be 0")
	}
	if len(m.Hash) == 0 {
		return errors.New("snapshot has no hash")
	}
	if m.Chunks == 0 {
		return errors.New("snapshot has no chunks")
	}
	return nil
}

// chunkRequestMessage requests a single chunk from a peer.
type chunkRequestMessage struct {
	Height uint64
	Format uint32
	Index  uint32
}

// ValidateBasic implements Message.
func (m *chunkRequestMessage) ValidateBasic() error {
	if m == nil {
		return errors.New("nil message")
	}
	if m.Height == 0 {
		return errors.New("height cannot be 0")
	}
	return nil
}

// chunkResponseMessage contains a single chunk from a peer.
type chunkResponseMessage struct {
	Height  uint64
	Format  uint32
	Index   uint32
	Chunk   []byte
	Missing bool
}

// ValidateBasic implements Message.
func (m *chunkResponseMessage) ValidateBasic() error {
	if m == nil {
		return errors.New("nil message")
	}
	if m.Height == 0 {
		return errors.New("height cannot be 0")
	}
	if m.Missing && len(m.Chunk) > 0 {
		return errors.New("missing chunk cannot have contents")
	}
	if !m.Missing && m.Chunk == nil {
		return errors.New("chunk cannot be nil")
	}
	return nil
}
