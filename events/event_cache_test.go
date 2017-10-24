package events

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventCache_Flush(t *testing.T) {
	evsw := NewEventSwitch()
	evsw.Start()
	evsw.AddListenerForEvent("nothingness", "", func(data EventData) {
		// Check we are not initialising an empty buffer full of zeroed eventInfos in the EventCache
		require.FailNow(t, "We should never receive a message on this switch since none are fired")
	})
	evc := NewEventCache(evsw)
	evc.Flush()
	// Check after reset
	evc.Flush()
	fail := true
	pass := false
	evsw.AddListenerForEvent("somethingness", "something", func(data EventData) {
		if fail {
			require.FailNow(t, "Shouldn't see a message until flushed")
		}
		pass = true
	})
	evc.FireEvent("something", struct{ int }{1})
	evc.FireEvent("something", struct{ int }{2})
	evc.FireEvent("something", struct{ int }{3})
	fail = false
	evc.Flush()
	assert.True(t, pass)
}
