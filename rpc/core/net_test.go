package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	rpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
)

func TestUnsafeDialSeeds(t *testing.T) {
	sw := p2p.MakeSwitch(cfg.DefaultP2PConfig(), 1, "testing", "123.123.123",
		func(n int, sw *p2p.Switch) *p2p.Switch { return sw })
	err := sw.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := sw.Stop(); err != nil {
			t.Error(err)
		}
	})

	env.Logger = log.TestingLogger()
	env.P2PPeers = sw

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
	sw.SetAddrBook(&p2p.AddrBookMock{
		Addrs:        make(map[string]struct{}),
		OurAddrs:     make(map[string]struct{}),
		PrivateAddrs: make(map[string]struct{}),
	})
	err := sw.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := sw.Stop(); err != nil {
			t.Error(err)
		}
	})

	env.Logger = log.TestingLogger()
	env.P2PPeers = sw

	testCases := []struct {
		peers                               []string
		persistence, unconditional, private bool
		isErr                               bool
	}{
		{[]string{}, false, false, false, true},
		{[]string{"d51fb70907db1c6c2d5237e78379b25cf1a37ab4@127.0.0.1:41198"}, true, true, true, false},
		{[]string{"127.0.0.1:41198"}, true, true, false, true},
	}

	for _, tc := range testCases {
		res, err := UnsafeDialPeers(&rpctypes.Context{}, tc.peers, tc.persistence, tc.unconditional, tc.private)
		if tc.isErr {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.NotNil(t, res)
		}
	}
}
