package p2p

import (
	"bytes"
	"fmt"
	"io"

	. "github.com/tendermint/tendermint/binary"
)

/*
A Message is anything that can be serialized.
The resulting serialized bytes of Message don't contain type information,
so messages are typically wrapped in a TypedMessage before put in the wire.
*/
type Message interface {
	Binary
}

/*
A TypedMessage extends a Message with a single byte of type information.
When deserializing a message from the wire, a switch statement is needed
to dispatch to the correct constructor, typically named "ReadXXXMessage".
*/
type TypedMessage struct {
	Type    Byte
	Message Message
}

func (tm TypedMessage) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteTo(tm.Type, w, n, err)
	n, err = WriteTo(tm.Message, w, n, err)
	return
}

func (tm TypedMessage) String() string {
	return fmt.Sprintf("0x%X⋺%v", tm.Type, tm.Message)
}

//-----------------------------------------------------------------------------

/*
Packet encapsulates a ByteSlice on a Channel.
Typically the Bytes are the serialized form of a TypedMessage.
*/
type Packet struct {
	Channel String
	Bytes   ByteSlice
	// Hash
}

func NewPacket(chName String, msg Binary) Packet {
	msgBytes := BinaryBytes(msg)
	return Packet{
		Channel: chName,
		Bytes:   msgBytes,
	}
}

func (p Packet) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteTo(p.Channel, w, n, err)
	n, err = WriteTo(p.Bytes, w, n, err)
	return
}

func (p Packet) Reader() io.Reader {
	return bytes.NewReader(p.Bytes)
}

func (p Packet) String() string {
	return fmt.Sprintf("%v:%X", p.Channel, p.Bytes)
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
	return Packet{Channel: chName, Bytes: bytes}, nil
}

/*
InboundPacket extends Packet with fields relevant to inbound packets.
*/
type InboundPacket struct {
	Peer *Peer
	Time Time
	Packet
}
