package types

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/crypto/ed25519"
	tmnet "github.com/tendermint/tendermint/libs/net"
	"github.com/tendermint/tendermint/version"
)

const testCh = 0x01

func TestNodeInfoValidate(t *testing.T) {

	// empty fails
	ni := NodeInfo{}
	assert.Error(t, ni.Validate())

	channels := make([]byte, maxNumChannels)
	for i := 0; i < maxNumChannels; i++ {
		channels[i] = byte(i)
	}
	dupChannels := make([]byte, 5)
	copy(dupChannels, channels[:5])
	dupChannels = append(dupChannels, testCh)

	nonASCII := "¢§µ"
	emptyTab := "\t"
	emptySpace := "  "

	testCases := []struct {
		testName         string
		malleateNodeInfo func(*NodeInfo)
		expectErr        bool
	}{
		{
			"Too Many Channels",
			func(ni *NodeInfo) { ni.Channels = append(channels, byte(maxNumChannels)) }, // nolint: gocritic
			true,
		},
		{"Duplicate Channel", func(ni *NodeInfo) { ni.Channels = dupChannels }, true},
		{"Good Channels", func(ni *NodeInfo) { ni.Channels = ni.Channels[:5] }, false},

		{"Invalid NetAddress", func(ni *NodeInfo) { ni.ListenAddr = "not-an-address" }, true},
		{"Good NetAddress", func(ni *NodeInfo) { ni.ListenAddr = "0.0.0.0:26656" }, false},

		{"Non-ASCII Version", func(ni *NodeInfo) { ni.Version = nonASCII }, true},
		{"Empty tab Version", func(ni *NodeInfo) { ni.Version = emptyTab }, true},
		{"Empty space Version", func(ni *NodeInfo) { ni.Version = emptySpace }, true},
		{"Empty Version", func(ni *NodeInfo) { ni.Version = "" }, false},

		{"Non-ASCII Moniker", func(ni *NodeInfo) { ni.Moniker = nonASCII }, true},
		{"Empty tab Moniker", func(ni *NodeInfo) { ni.Moniker = emptyTab }, true},
		{"Empty space Moniker", func(ni *NodeInfo) { ni.Moniker = emptySpace }, true},
		{"Empty Moniker", func(ni *NodeInfo) { ni.Moniker = "" }, true},
		{"Good Moniker", func(ni *NodeInfo) { ni.Moniker = "hey its me" }, false},

		{"Non-ASCII TxIndex", func(ni *NodeInfo) { ni.Other.TxIndex = nonASCII }, true},
		{"Empty tab TxIndex", func(ni *NodeInfo) { ni.Other.TxIndex = emptyTab }, true},
		{"Empty space TxIndex", func(ni *NodeInfo) { ni.Other.TxIndex = emptySpace }, true},
		{"Empty TxIndex", func(ni *NodeInfo) { ni.Other.TxIndex = "" }, false},
		{"Off TxIndex", func(ni *NodeInfo) { ni.Other.TxIndex = "off" }, false},

		{"Non-ASCII RPCAddress", func(ni *NodeInfo) { ni.Other.RPCAddress = nonASCII }, true},
		{"Empty tab RPCAddress", func(ni *NodeInfo) { ni.Other.RPCAddress = emptyTab }, true},
		{"Empty space RPCAddress", func(ni *NodeInfo) { ni.Other.RPCAddress = emptySpace }, true},
		{"Empty RPCAddress", func(ni *NodeInfo) { ni.Other.RPCAddress = "" }, false},
		{"Good RPCAddress", func(ni *NodeInfo) { ni.Other.RPCAddress = "0.0.0.0:26657" }, false},
	}

	nodeKeyID := testNodeID()
	name := "testing"

	// test case passes
	ni = testNodeInfo(nodeKeyID, name)
	ni.Channels = channels
	assert.NoError(t, ni.Validate())

	for _, tc := range testCases {
		ni := testNodeInfo(nodeKeyID, name)
		ni.Channels = channels
		tc.malleateNodeInfo(&ni)
		err := ni.Validate()
		if tc.expectErr {
			assert.Error(t, err, tc.testName)
		} else {
			assert.NoError(t, err, tc.testName)
		}
	}

}

func testNodeID() NodeID {
	return NodeIDFromPubKey(ed25519.GenPrivKey().PubKey())
}

func testNodeInfo(id NodeID, name string) NodeInfo {
	return testNodeInfoWithNetwork(id, name, "testing")
}

func testNodeInfoWithNetwork(id NodeID, name, network string) NodeInfo {
	return NodeInfo{
		ProtocolVersion: ProtocolVersion{
			P2P:   version.P2PProtocol,
			Block: version.BlockProtocol,
			App:   0,
		},
		NodeID:     id,
		ListenAddr: fmt.Sprintf("127.0.0.1:%d", getFreePort()),
		Network:    network,
		Version:    "1.2.3-rc0-deadbeef",
		Channels:   []byte{testCh},
		Moniker:    name,
		Other: NodeInfoOther{
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

func TestNodeInfoCompatible(t *testing.T) {
	nodeKey1ID := testNodeID()
	nodeKey2ID := testNodeID()
	name := "testing"

	var newTestChannel byte = 0x2

	// test NodeInfo is compatible
	ni1 := testNodeInfo(nodeKey1ID, name)
	ni2 := testNodeInfo(nodeKey2ID, name)
	assert.NoError(t, ni1.CompatibleWith(ni2))

	// add another channel; still compatible
	ni2.Channels = []byte{newTestChannel, testCh}
	assert.NoError(t, ni1.CompatibleWith(ni2))

	testCases := []struct {
		testName         string
		malleateNodeInfo func(*NodeInfo)
	}{
		{"Wrong block version", func(ni *NodeInfo) { ni.ProtocolVersion.Block++ }},
		{"Wrong network", func(ni *NodeInfo) { ni.Network += "-wrong" }},
		{"No common channels", func(ni *NodeInfo) { ni.Channels = []byte{newTestChannel} }},
	}

	for _, tc := range testCases {
		ni := testNodeInfo(nodeKey2ID, name)
		tc.malleateNodeInfo(&ni)
		assert.Error(t, ni1.CompatibleWith(ni))
	}
}

func TestNodeInfoAddChannel(t *testing.T) {
	nodeInfo := testNodeInfo(testNodeID(), "testing")
	nodeInfo.Channels = []byte{}
	require.Empty(t, nodeInfo.Channels)

	nodeInfo.AddChannel(2)
	require.Contains(t, nodeInfo.Channels, byte(0x02))

	// adding the same channel again shouldn't be a problem
	nodeInfo.AddChannel(2)
	require.Contains(t, nodeInfo.Channels, byte(0x02))
}
