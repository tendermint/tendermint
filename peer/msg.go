package peer

import (
	"io"

	. "github.com/tendermint/tendermint/binary"
)

/*
Packet encapsulates a ByteSlice on a Channel.
*/
type Packet struct {
	Channel String
	Bytes   ByteSlice
	// Hash
}

func NewPacket(chName String, bytes ByteSlice) Packet {
	return Packet{
		Channel: chName,
		Bytes:   bytes,
	}
}

func (p Packet) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteOnto(&p.Channel, w, n, err)
	n, err = WriteOnto(&p.Bytes, w, n, err)
	return
}

func ReadPacketSafe(r io.Reader) (pkt Packet, err error) {
	chName, err := ReadStringSafe(r)
	if err != nil {
		return
	}
	// TODO: packet length sanity check.
	bytes, err := ReadByteSliceSafe(r)
	if err != nil {
		return
	}
	return NewPacket(chName, bytes), nil
}

/*
InboundPacket extends Packet with fields relevant to incoming packets.
*/
type InboundPacket struct {
	Peer *Peer
	Time Time
	Packet
}

/*
NewFilterMsg is not implemented. TODO
*/
type NewFilterMsg struct {
	ChName String
	Filter interface{} // todo
}

func (m *NewFilterMsg) WriteTo(w io.Writer) (int64, error) {
	panic("TODO: implement")
	return 0, nil // TODO
}
