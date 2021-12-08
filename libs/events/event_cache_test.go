package events

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventCache_Flush(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	evsw := NewEventSwitch()
	err := evsw.Start(ctx)
	require.NoError(t, err)

	err = evsw.AddListenerForEvent("nothingness", "", func(_ context.Context, data EventData) error {
		// Check we are not initializing an empty buffer full of zeroed eventInfos in the EventCache
		require.FailNow(t, "We should never receive a message on this switch since none are fired")
		return nil
	})
	require.NoError(t, err)

	evc := NewEventCache(evsw)
	evc.Flush(ctx)
	// Check after reset
	evc.Flush(ctx)
	fail := true
	pass := false
	err = evsw.AddListenerForEvent("somethingness", "something", func(_ context.Context, data EventData) error {
		if fail {
			require.FailNow(t, "Shouldn't see a message until flushed")
		}
		pass = true
		return nil
	})
	require.NoError(t, err)

	evc.FireEvent("something", struct{ int }{1})
	evc.FireEvent("something", struct{ int }{2})
	evc.FireEvent("something", struct{ int }{3})
	fail = false
	evc.Flush(ctx)
	assert.True(t, pass)
}
