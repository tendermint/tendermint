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
	assert.Error(t, ni.ValidateBasic())

	channels := make([]byte, maxNumChannels)
	for i := 0; i < maxNumChannels; i++ {
		channels[i] = byte(i)
	}
	nonAscii := "¢§µ"
	emptyTab := fmt.Sprintf("\t")
	emptySpace := fmt.Sprintf("  ")

	testCases := []struct {
		testName         string
		malleateNodeInfo func(*DefaultNodeInfo)
	}{
		{"Too Many Channels", func(ni *DefaultNodeInfo) { ni.Channels = append(channels, byte(maxNumChannels)) }},
		{"Duplicate Channel", func(ni *DefaultNodeInfo) { ni.Channels = append(channels[:5], 0x1) }},
		{"Invalid NetAddress", func(ni *DefaultNodeInfo) { ni.ListenAddr = "not-an-address" }},

		{"Non-ASCII Moniker", func(ni *DefaultNodeInfo) { ni.Moniker = nonAscii }},
		{"Empty tab Moniker", func(ni *DefaultNodeInfo) { ni.Moniker = emptyTab }},
		{"Empty space Moniker", func(ni *DefaultNodeInfo) { ni.Moniker = emptySpace }},
		{"Empty Moniker", func(ni *DefaultNodeInfo) { ni.Moniker = "" }},

		{"Non-ASCII TxIndex", func(ni *DefaultNodeInfo) { ni.Other.TxIndex = nonAscii }},
		{"Empty tab TxIndex", func(ni *DefaultNodeInfo) { ni.Other.TxIndex = emptyTab }},
		{"Empty space TxIndex", func(ni *DefaultNodeInfo) { ni.Other.TxIndex = emptySpace }},

		{"Non-ASCII RPCAddress", func(ni *DefaultNodeInfo) { ni.Other.RPCAddress = nonAscii }},
		{"Empty tab RPCAddress", func(ni *DefaultNodeInfo) { ni.Other.RPCAddress = emptyTab }},
		{"Empty space RPCAddress", func(ni *DefaultNodeInfo) { ni.Other.RPCAddress = emptySpace }},
	}

	nodeKey := NodeKey{PrivKey: ed25519.GenPrivKey()}
	name := "testing"

	// test case passes
	ni = testNodeInfo(nodeKey.ID(), name).(DefaultNodeInfo)
	ni.Channels = channels
	assert.NoError(t, ni.ValidateBasic())

	for _, tc := range testCases {
		ni := testNodeInfo(nodeKey.ID(), name).(DefaultNodeInfo)
		ni.Channels = channels
		tc.malleateNodeInfo(&ni)
		assert.Error(t, ni.ValidateBasic())
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
		{"Wrong block version", func(ni *DefaultNodeInfo) { ni.Version.Block += 1 }},
		{"Wrong network", func(ni *DefaultNodeInfo) { ni.Network += "-wrong" }},
		{"No common channels", func(ni *DefaultNodeInfo) { ni.Channels = []byte{newTestChannel} }},
	}

	for _, tc := range testCases {
		ni := testNodeInfo(nodeKey2.ID(), name).(DefaultNodeInfo)
		tc.malleateNodeInfo(&ni)
		assert.Error(t, ni1.CompatibleWith(ni))
	}
}
