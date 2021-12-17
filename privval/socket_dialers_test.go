package privval

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto/ed25519"
	tmnet "github.com/tendermint/tendermint/libs/net"
)

// GetFreeLocalhostAddrPort returns a free localhost:port address
func GetFreeLocalhostAddrPort(t *testing.T) string {
	port, err := tmnet.GetFreePort()
	require.NoError(t, err)

	return fmt.Sprintf("127.0.0.1:%d", port)
}

func getDialerTestCases(t *testing.T) []dialerTestCase {
	tcpAddr := GetFreeLocalhostAddrPort(t)
	unixFilePath, err := testUnixAddr()
	require.NoError(t, err)
	unixAddr := fmt.Sprintf("unix://%s", unixFilePath)

	return []dialerTestCase{
		{
			addr:   tcpAddr,
			dialer: DialTCPFn(tcpAddr, testTimeoutReadWrite, ed25519.GenPrivKey()),
		},
		{
			addr:   unixAddr,
			dialer: DialUnixFn(unixFilePath),
		},
	}
}

func TestIsConnTimeoutForFundamentalTimeouts(t *testing.T) {
	// Generate a networking timeout
	tcpAddr := GetFreeLocalhostAddrPort(t)
	dialer := DialTCPFn(tcpAddr, time.Millisecond, ed25519.GenPrivKey())
	_, err := dialer()
	assert.Error(t, err)
	assert.True(t, IsConnTimeout(err))
}

func TestIsConnTimeoutForWrappedConnTimeouts(t *testing.T) {
	tcpAddr := GetFreeLocalhostAddrPort(t)
	dialer := DialTCPFn(tcpAddr, time.Millisecond, ed25519.GenPrivKey())
	_, err := dialer()
	assert.Error(t, err)
	err = fmt.Errorf("%v: %w", err, ErrConnectionTimeout)
	assert.True(t, IsConnTimeout(err))
}
