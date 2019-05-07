package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	rpctypes "github.com/tendermint/tendermint/rpc/lib/types"
)

func TestUnsafeDialSeeds(t *testing.T) {
	sw := p2p.MakeSwitch(cfg.DefaultP2PConfig(), 1, "testing", "123.123.123",
		func(n int, sw *p2p.Switch) *p2p.Switch { return sw })
	err := sw.Start()
	require.NoError(t, err)
	defer sw.Stop()

	logger = log.TestingLogger()
	p2pPeers = sw

	testCases := []struct {
		seeds []string
		isErr bool
	}{
		{[]string{}, true},
		{[]string{"d51fb70907db1c6c2d5237e78379b25cf1a37ab4@127.0.0.1:41198"}, false},
		{[]string{"127.0.0.1:41198"}, true},
	}

	for _, tc := range testCases {
		res, err := UnsafeDialSeeds(&rpctypes.Context{}, tc.seeds)
		if tc.isErr {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.NotNil(t, res)
		}
	}
}

func TestUnsafeDialPeers(t *testing.T) {
	sw := p2p.MakeSwitch(cfg.DefaultP2PConfig(), 1, "testing", "123.123.123",
		func(n int, sw *p2p.Switch) *p2p.Switch { return sw })
	err := sw.Start()
	require.NoError(t, err)
	defer sw.Stop()

	logger = log.TestingLogger()
	p2pPeers = sw

	testCases := []struct {
		peers []string
		isErr bool
	}{
		{[]string{}, true},
		{[]string{"d51fb70907db1c6c2d5237e78379b25cf1a37ab4@127.0.0.1:41198"}, false},
		{[]string{"127.0.0.1:41198"}, true},
	}

	for _, tc := range testCases {
		res, err := UnsafeDialPeers(&rpctypes.Context{}, tc.peers, false)
		if tc.isErr {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.NotNil(t, res)
		}
	}
}
