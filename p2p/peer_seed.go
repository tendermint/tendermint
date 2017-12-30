package p2p

import (
	"bytes"
	"fmt"
	"strings"

	lpeer "github.com/libp2p/go-libp2p-peer"
	ps "github.com/libp2p/go-libp2p-peerstore"
	ma "github.com/multiformats/go-multiaddr"
)

// Seed implements the CLI seed formatting.
// Example: QmYtUc4iTCbbfVSDNKvtQqrfyezPPnFvE33wFmutw9PBBk@192.168.1.1:5000
// Example: QmYtUc4iTCbbfVSDNKvtQqrfyezPPnFvE33wFmutw9PBBk@/ip4/192.168.1.1/tcp/5000
type Seed string

// ParsePeerInfo attempts to unmarshal the seed to a PeerInfo object.
func (s Seed) ParsePeerInfo() (peerInfo ps.PeerInfo, err error) {
	setDefErr := func() {
		err = fmt.Errorf("invalid seed format (expected peerid@ip1:port+ip2:port...): %s", s)
	}

	seedStr := string(s)
	seedStr = strings.TrimSpace(seedStr)
	seedPts := strings.Split(seedStr, "@")

	// Attempt to parse first part to a peer ID.
	peerInfo.ID, err = lpeer.IDB58Decode(seedPts[0])
	if err != nil {
		setDefErr()
		return
	}

	if len(seedPts) == 1 {
		return
	}

	// Split addresses
	addrs := strings.Split(seedPts[1], "+")
	for _, addr := range addrs {
		var maddr ma.Multiaddr
		maddr, err = ParseMultiaddrFlexible(addr)
		if err != nil {
			setDefErr()
			return
		}

		if maddr != nil {
			peerInfo.Addrs = append(peerInfo.Addrs, maddr)
		}
	}

	return
}

// NewSeed builds a new seed.
func NewSeed(peerInfo ps.PeerInfo) Seed {
	out := &bytes.Buffer{}
	out.WriteString(peerInfo.ID.Pretty())
	for i, addr := range peerInfo.Addrs {
		if i == 0 {
			out.WriteRune('@')
		} else {
			out.WriteRune('+')
		}
		out.WriteString(addr.String())
	}
	return Seed(out.String())
}
