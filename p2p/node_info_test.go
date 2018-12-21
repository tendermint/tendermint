package p2p

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tendermint/tendermint/crypto/ed25519"
)

func TestNodeInfoValidate(t *testing.T) {

	// empty fails
	ni := DefaultNodeInfo{}
	assert.Error(t, ni.Validate())

	channels := make([]byte, maxNumChannels)
	for i := 0; i < maxNumChannels; i++ {
		channels[i] = byte(i)
	}
	dupChannels := make([]byte, 5)
	copy(dupChannels[:], channels[:5])
	dupChannels = append(dupChannels, testCh)

	nonAscii := "¢§µ"
	emptyTab := fmt.Sprintf("\t")
	emptySpace := fmt.Sprintf("  ")

	testCases := []struct {
		testName         string
		malleateNodeInfo func(*DefaultNodeInfo)
		expectErr        bool
	}{
		{"Too Many Channels", func(ni *DefaultNodeInfo) { ni.Channels = append(channels, byte(maxNumChannels)) }, true},
		{"Duplicate Channel", func(ni *DefaultNodeInfo) { ni.Channels = dupChannels }, true},
		{"Good Channels", func(ni *DefaultNodeInfo) { ni.Channels = ni.Channels[:5] }, false},

		{"Invalid NetAddress", func(ni *DefaultNodeInfo) { ni.ListenAddr = "not-an-address" }, true},
		{"Good NetAddress", func(ni *DefaultNodeInfo) { ni.ListenAddr = "0.0.0.0:26656" }, false},

		{"Non-ASCII Version", func(ni *DefaultNodeInfo) { ni.Version = nonAscii }, true},
		{"Empty tab Version", func(ni *DefaultNodeInfo) { ni.Version = emptyTab }, true},
		{"Empty space Version", func(ni *DefaultNodeInfo) { ni.Version = emptySpace }, true},
		{"Empty Version", func(ni *DefaultNodeInfo) { ni.Version = "" }, false},

		{"Non-ASCII Moniker", func(ni *DefaultNodeInfo) { ni.Moniker = nonAscii }, true},
		{"Empty tab Moniker", func(ni *DefaultNodeInfo) { ni.Moniker = emptyTab }, true},
		{"Empty space Moniker", func(ni *DefaultNodeInfo) { ni.Moniker = emptySpace }, true},
		{"Empty Moniker", func(ni *DefaultNodeInfo) { ni.Moniker = "" }, true},
		{"Good Moniker", func(ni *DefaultNodeInfo) { ni.Moniker = "hey its me" }, false},

		{"Non-ASCII TxIndex", func(ni *DefaultNodeInfo) { ni.Other.TxIndex = nonAscii }, true},
		{"Empty tab TxIndex", func(ni *DefaultNodeInfo) { ni.Other.TxIndex = emptyTab }, true},
		{"Empty space TxIndex", func(ni *DefaultNodeInfo) { ni.Other.TxIndex = emptySpace }, true},
		{"Empty TxIndex", func(ni *DefaultNodeInfo) { ni.Other.TxIndex = "" }, false},
		{"Off TxIndex", func(ni *DefaultNodeInfo) { ni.Other.TxIndex = "off" }, false},

		{"Non-ASCII RPCAddress", func(ni *DefaultNodeInfo) { ni.Other.RPCAddress = nonAscii }, true},
		{"Empty tab RPCAddress", func(ni *DefaultNodeInfo) { ni.Other.RPCAddress = emptyTab }, true},
		{"Empty space RPCAddress", func(ni *DefaultNodeInfo) { ni.Other.RPCAddress = emptySpace }, true},
		{"Empty RPCAddress", func(ni *DefaultNodeInfo) { ni.Other.RPCAddress = "" }, false},
		{"Good RPCAddress", func(ni *DefaultNodeInfo) { ni.Other.RPCAddress = "0.0.0.0:26657" }, false},
	}

	nodeKey := NodeKey{PrivKey: ed25519.GenPrivKey()}
	name := "testing"

	// test case passes
	ni = testNodeInfo(nodeKey.ID(), name).(DefaultNodeInfo)
	ni.Channels = channels
	assert.NoError(t, ni.Validate())

	for _, tc := range testCases {
		ni := testNodeInfo(nodeKey.ID(), name).(DefaultNodeInfo)
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

func TestNodeInfoCompatible(t *testing.T) {

	nodeKey1 := NodeKey{PrivKey: ed25519.GenPrivKey()}
	nodeKey2 := NodeKey{PrivKey: ed25519.GenPrivKey()}
	name := "testing"

	var newTestChannel byte = 0x2

	// test NodeInfo is compatible
	ni1 := testNodeInfo(nodeKey1.ID(), name).(DefaultNodeInfo)
	ni2 := testNodeInfo(nodeKey2.ID(), name).(DefaultNodeInfo)
	assert.NoError(t, ni1.CompatibleWith(ni2))

	// add another channel; still compatible
	ni2.Channels = []byte{newTestChannel, testCh}
	assert.NoError(t, ni1.CompatibleWith(ni2))

	// wrong NodeInfo type is not compatible
	_, netAddr := CreateRoutableAddr()
	ni3 := mockNodeInfo{netAddr}
	assert.Error(t, ni1.CompatibleWith(ni3))

	testCases := []struct {
		testName         string
		malleateNodeInfo func(*DefaultNodeInfo)
	}{
		{"Wrong block version", func(ni *DefaultNodeInfo) { ni.ProtocolVersion.Block += 1 }},
		{"Wrong network", func(ni *DefaultNodeInfo) { ni.Network += "-wrong" }},
		{"No common channels", func(ni *DefaultNodeInfo) { ni.Channels = []byte{newTestChannel} }},
	}

	for _, tc := range testCases {
		ni := testNodeInfo(nodeKey2.ID(), name).(DefaultNodeInfo)
		tc.malleateNodeInfo(&ni)
		assert.Error(t, ni1.CompatibleWith(ni))
	}
}
