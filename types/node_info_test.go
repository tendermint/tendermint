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
	ni = testNodeInfo(t, nodeKeyID, name)
	ni.Channels = channels
	assert.NoError(t, ni.Validate())

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			ni := testNodeInfo(t, nodeKeyID, name)
			ni.Channels = channels
			tc.malleateNodeInfo(&ni)
			err := ni.Validate()
			if tc.expectErr {
				assert.Error(t, err, tc.testName)
			} else {
				assert.NoError(t, err, tc.testName)
			}
		})

	}

}

func testNodeID() NodeID {
	return NodeIDFromPubKey(ed25519.GenPrivKey().PubKey())
}

func testNodeInfo(t *testing.T, id NodeID, name string) NodeInfo {
	return testNodeInfoWithNetwork(t, id, name, "testing")
}

func testNodeInfoWithNetwork(t *testing.T, id NodeID, name, network string) NodeInfo {
	t.Helper()
	return NodeInfo{
		ProtocolVersion: ProtocolVersion{
			P2P:   version.P2PProtocol,
			Block: version.BlockProtocol,
			App:   0,
		},
		NodeID:     id,
		ListenAddr: fmt.Sprintf("127.0.0.1:%d", getFreePort(t)),
		Network:    network,
		Version:    "1.2.3-rc0-deadbeef",
		Channels:   []byte{testCh},
		Moniker:    name,
		Other: NodeInfoOther{
			TxIndex:    "on",
			RPCAddress: fmt.Sprintf("127.0.0.1:%d", getFreePort(t)),
		},
	}
}

func getFreePort(t *testing.T) int {
	t.Helper()
	port, err := tmnet.GetFreePort()
	require.NoError(t, err)
	return port
}

func TestNodeInfoCompatible(t *testing.T) {
	nodeKey1ID := testNodeID()
	nodeKey2ID := testNodeID()
	name := "testing"

	var newTestChannel byte = 0x2

	// test NodeInfo is compatible
	ni1 := testNodeInfo(t, nodeKey1ID, name)
	ni2 := testNodeInfo(t, nodeKey2ID, name)
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
		ni := testNodeInfo(t, nodeKey2ID, name)
		tc.malleateNodeInfo(&ni)
		assert.Error(t, ni1.CompatibleWith(ni))
	}
}

func TestNodeInfoAddChannel(t *testing.T) {
	nodeInfo := testNodeInfo(t, testNodeID(), "testing")
	nodeInfo.Channels = []byte{}
	require.Empty(t, nodeInfo.Channels)

	nodeInfo.AddChannel(2)
	require.Contains(t, nodeInfo.Channels, byte(0x02))

	// adding the same channel again shouldn't be a problem
	nodeInfo.AddChannel(2)
	require.Contains(t, nodeInfo.Channels, byte(0x02))
}

func TestParseAddressString(t *testing.T) {
	testCases := []struct {
		name     string
		addr     string
		expected string
		correct  bool
	}{
		{"no node id and no protocol", "127.0.0.1:8080", "", false},
		{"no node id w/ tcp input", "tcp://127.0.0.1:8080", "", false},
		{"no node id w/ udp input", "udp://127.0.0.1:8080", "", false},

		{
			"no protocol",
			"deadbeefdeadbeefdeadbeefdeadbeefdeadbeef@127.0.0.1:8080",
			"deadbeefdeadbeefdeadbeefdeadbeefdeadbeef@127.0.0.1:8080",
			true,
		},
		{
			"tcp input",
			"tcp://deadbeefdeadbeefdeadbeefdeadbeefdeadbeef@127.0.0.1:8080",
			"deadbeefdeadbeefdeadbeefdeadbeefdeadbeef@127.0.0.1:8080",
			true,
		},
		{
			"udp input",
			"udp://deadbeefdeadbeefdeadbeefdeadbeefdeadbeef@127.0.0.1:8080",
			"deadbeefdeadbeefdeadbeefdeadbeefdeadbeef@127.0.0.1:8080",
			true,
		},
		{"malformed tcp input", "tcp//deadbeefdeadbeefdeadbeefdeadbeefdeadbeef@127.0.0.1:8080", "", false},
		{"malformed udp input", "udp//deadbeefdeadbeefdeadbeefdeadbeefdeadbeef@127.0.0.1:8080", "", false},

		// {"127.0.0:8080", false},
		{"invalid host", "notahost", "", false},
		{"invalid port", "127.0.0.1:notapath", "", false},
		{"invalid host w/ port", "notahost:8080", "", false},
		{"just a port", "8082", "", false},
		{"non-existent port", "127.0.0:8080000", "", false},

		{"too short nodeId", "deadbeef@127.0.0.1:8080", "", false},
		{"too short, not hex nodeId", "this-isnot-hex@127.0.0.1:8080", "", false},
		{"not hex nodeId", "xxxxbeefdeadbeefdeadbeefdeadbeefdeadbeef@127.0.0.1:8080", "", false},

		{"too short nodeId w/tcp", "tcp://deadbeef@127.0.0.1:8080", "", false},
		{"too short notHex nodeId w/tcp", "tcp://this-isnot-hex@127.0.0.1:8080", "", false},
		{"notHex nodeId w/tcp", "tcp://xxxxbeefdeadbeefdeadbeefdeadbeefdeadbeef@127.0.0.1:8080", "", false},
		{
			"correct nodeId w/tcp",
			"tcp://deadbeefdeadbeefdeadbeefdeadbeefdeadbeef@127.0.0.1:8080",
			"deadbeefdeadbeefdeadbeefdeadbeefdeadbeef@127.0.0.1:8080",
			true,
		},

		{"no node id", "tcp://@127.0.0.1:8080", "", false},
		{"no node id or IP", "tcp://@", "", false},
		{"tcp no host, w/ port", "tcp://:26656", "", false},
		{"empty", "", "", false},
		{"node id delimiter 1", "@", "", false},
		{"node id delimiter 2", " @", "", false},
		{"node id delimiter 3", " @ ", "", false},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			addr, port, err := ParseAddressString(tc.addr)
			if tc.correct {
				require.NoError(t, err, tc.addr)
				assert.Contains(t, tc.expected, addr.String())
				assert.Contains(t, tc.expected, fmt.Sprint(port))
			} else {
				assert.Error(t, err, "%v", tc.addr)
			}
		})
	}
}
