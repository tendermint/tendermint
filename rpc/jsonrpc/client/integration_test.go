//go:build release
// +build release

// The code in here is comprehensive as an integration
// test and is long, hence is only run before releases.

package client

import (
	"bytes"
	"context"
	"errors"
	"net"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWSClientReconnectWithJitter(t *testing.T) {
	const numClients = 8
	const maxReconnectAttempts = 3
	const maxSleepTime = time.Duration(((1<<maxReconnectAttempts)-1)+maxReconnectAttempts) * time.Second

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	failDialer := func(net, addr string) (net.Conn, error) {
		return nil, errors.New("not connected")
	}

	clientMap := make(map[int]*WSClient)
	buf := new(bytes.Buffer)
	for i := 0; i < numClients; i++ {
		c, err := NewWS("tcp://foo", "/websocket")
		require.NoError(t, err)
		c.Dialer = failDialer
		c.maxReconnectAttempts = maxReconnectAttempts
		c.Start(ctx)

		// Not invoking defer c.Stop() because
		// after all the reconnect attempts have been
		// exhausted, c.Stop is implicitly invoked.
		clientMap[i] = c
		// Trigger the reconnect routine that performs exponential backoff.
		go c.reconnect(ctx)
	}

	// Next we have to examine the logs to ensure that no single time was repeated
	backoffDurRegexp := regexp.MustCompile(`backoff_duration=(.+)`)
	matches := backoffDurRegexp.FindAll(buf.Bytes(), -1)
	seenMap := make(map[string]int)
	for i, match := range matches {
		if origIndex, seen := seenMap[string(match)]; seen {
			t.Errorf("match #%d (%q) was seen originally at log entry #%d", i, match, origIndex)
		} else {
			seenMap[string(match)] = i
		}
	}
}
