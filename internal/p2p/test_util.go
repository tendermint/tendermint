package p2p

import (
	"fmt"
	mrand "math/rand"

	tmnet "github.com/tendermint/tendermint/libs/net"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/types"
)

const testCh = 0x01

//------------------------------------------------

// nolint:gosec // G404: Use of weak random number generator
func CreateRoutableAddr() (addr string, netAddr *NetAddress) {
	for {
		var err error
		addr = fmt.Sprintf("%X@%v.%v.%v.%v:26656",
			tmrand.Bytes(20),
			mrand.Int()%256,
			mrand.Int()%256,
			mrand.Int()%256,
			mrand.Int()%256)
		netAddr, err = types.NewNetAddressString(addr)
		if err != nil {
			panic(err)
		}
		if netAddr.Routable() {
			break
		}
	}
	return
}

//----------------------------------------------------------------
// rand node info

func testNodeInfo(id types.NodeID, name string) types.NodeInfo {
	return testNodeInfoWithNetwork(id, name, "testing")
}

func testNodeInfoWithNetwork(id types.NodeID, name, network string) types.NodeInfo {
	return types.NodeInfo{
		ProtocolVersion: defaultProtocolVersion,
		NodeID:          id,
		ListenAddr:      fmt.Sprintf("127.0.0.1:%d", getFreePort()),
		Network:         network,
		Version:         "1.2.3-rc0-deadbeef",
		Channels:        []byte{testCh},
		Moniker:         name,
		Other: types.NodeInfoOther{
			TxIndex:    "on",
			RPCAddress: fmt.Sprintf("127.0.0.1:%d", getFreePort()),
		},
	}
}

func getFreePort() int {
	port, err := tmnet.GetFreePort()
	if err != nil {
		panic(err)
	}
	return port
}
