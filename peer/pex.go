package peer

import (
	"bytes"
	"errors"
	"io"

	. "github.com/tendermint/tendermint/binary"
)

var pexErrInvalidMessage = errors.New("Invalid PEX message")

const pexCh = "PEX"

func peerExchangeHandler(c *Client) {

	for {
		inPkt := c.Receive(pexCh) // {Peer, Time, Packet}
		if inPkt == nil {
			// Client has stopped
			break
		}

		// decode message
		msg := decodeMessage(inPkt.Bytes)

		switch msg.(type) {
		case *pexRequestMessage:
			// inPkt.Peer requested some peers.
			// TODO: prevent abuse.
			addrs := c.addrBook.GetSelection()
			response := &pexResponseMessage{Addrs: addrs}
			pkt := NewPacket(pexCh, BinaryBytes(response))
			queued := inPkt.Peer.TryQueue(pkt)
			if !queued {
				// ignore
			}
		case *pexResponseMessage:
			// We received some peer addresses from inPkt.Peer.
			// TODO: prevent abuse.
			// (We don't want to get spammed with bad peers)
			srcAddr := inPkt.Peer.RemoteAddress()
			for _, addr := range msg.(*pexResponseMessage).Addrs {
				c.addrBook.AddAddress(addr, srcAddr)
			}
		default:
			// Bad peer.
			c.StopPeerForError(inPkt.Peer, pexErrInvalidMessage)
		}
	}

	// cleanup

}

/* Messages */

const (
	pexTypeUnknown  = Byte(0x00)
	pexTypeRequest  = Byte(0x01)
	pexTypeResponse = Byte(0x02)
)

// TODO: check for unnecessary extra bytes at the end.
func decodeMessage(bz ByteSlice) (msg Message) {
	switch Byte(bz[0]) {
	case pexTypeRequest:
		return &pexRequestMessage{}
	case pexTypeResponse:
		return readPexResponseMessage(bytes.NewReader(bz[1:]))
	default:
		return nil
	}
}

/*
A response with peer addresses
*/
type pexRequestMessage struct {
}

func (m *pexRequestMessage) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteOnto(pexTypeRequest, w, n, err)
	return
}

/*
A response with peer addresses
*/
type pexResponseMessage struct {
	Addrs []*NetAddress
}

func readPexResponseMessage(r io.Reader) *pexResponseMessage {
	numAddrs := int(ReadUInt32(r))
	addrs := []*NetAddress{}
	for i := 0; i < numAddrs; i++ {
		addr := ReadNetAddress(r)
		addrs = append(addrs, addr)
	}
	return &pexResponseMessage{
		Addrs: addrs,
	}
}

func (m *pexResponseMessage) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteOnto(pexTypeResponse, w, n, err)
	n, err = WriteOnto(UInt32(len(m.Addrs)), w, n, err)
	for _, addr := range m.Addrs {
		n, err = WriteOnto(addr, w, n, err)
	}
	return
}
