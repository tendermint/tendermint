package p2p

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	. "github.com/tendermint/go-common"
)

func TestBasic(t *testing.T) {
	book := NewAddrBook(createTempFileName("addrbook"), true)
	r := NewPEXReactor(book)

	assert.NotNil(t, r)
	assert.NotEmpty(t, r.GetChannels())
}

func TestAddRemovePeer(t *testing.T) {
	book := NewAddrBook(createTempFileName("addrbook"), true)
	r := NewPEXReactor(book)

	size := book.Size()
	peer := createRandomPeer(false)

	r.AddPeer(peer)
	assert.Equal(t, size+1, book.Size())

	r.RemovePeer(peer, "peer not available")
	assert.Equal(t, size, book.Size())

	outboundPeer := createRandomPeer(true)

	r.AddPeer(outboundPeer)
	assert.Equal(t, size, book.Size(), "size must not change")

	r.RemovePeer(outboundPeer, "peer not available")
	assert.Equal(t, size, book.Size(), "size must not change")
}

func createRandomPeer(outbound bool) *Peer {
	return &Peer{
		Key: RandStr(12),
		NodeInfo: &NodeInfo{
			RemoteAddr: Fmt("%v.%v.%v.%v:46656", rand.Int()%256, rand.Int()%256, rand.Int()%256, rand.Int()%256),
			ListenAddr: Fmt("%v.%v.%v.%v:46656", rand.Int()%256, rand.Int()%256, rand.Int()%256, rand.Int()%256),
		},
		outbound: outbound,
	}
}
