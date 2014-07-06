package peer

import (
	. "github.com/tendermint/tendermint/binary"
	"io"
)

const pexCh = "PEX"

func peerExchangeHandler(c *Client) {

	for {
		// inPkt := c.Receive(pexCh) // {Peer, Time, Packet}

		// decode message

		// if message is a peer request

		// if message is
	}

	// cleanup

}

/* Messages */

const (
	pexTypeRequest  = Byte(0x00)
	pexTypeResponse = Byte(0x01)
)

func decodeMsg(bytes ByteSlice) (t Byte, msg Message) {
	//return pexTypeRequest, nil
	return pexTypeResponse, nil
}

/*
A response with peer addresses
*/
type pexResponseMsg struct {
	Addrs []*NetAddress
}

func readPexResponseMsg(r io.Reader) *pexResponseMsg {
	numAddrs := int(ReadUInt32(r))
	addrs := []*NetAddress{}
	for i := 0; i < numAddrs; i++ {
		addr := ReadNetAddress(r)
		addrs = append(addrs, addr)
	}
	return &pexResponseMsg{
		Addrs: addrs,
	}
}

func (m *pexResponseMsg) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteOnto(UInt32(len(m.Addrs)), w, n, err)
	for _, addr := range m.Addrs {
		n, err = WriteOnto(addr, w, n, err)
	}
	return
}

func (m *pexResponseMsg) Type() string {
	return "pexTypeResponse"
}
