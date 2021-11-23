package sync_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	tmsync "github.com/tendermint/tendermint/internal/libs/sync"
)

func TestWaker(t *testing.T) {

	// A new waker should block when sleeping.
	waker := tmsync.NewWaker()

	select {
	case <-waker.Sleep():
		require.Fail(t, "unexpected wakeup")
	default:
	}

	// Wakeups should not block, and should cause the next sleeper to awaken.
	waker.Wake()

	select {
	case <-waker.Sleep():
	default:
		require.Fail(t, "expected wakeup, but sleeping instead")
	}

	// Multiple wakeups should only wake a single sleeper.
	waker.Wake()
	waker.Wake()
	waker.Wake()

	select {
	case <-waker.Sleep():
	default:
		require.Fail(t, "expected wakeup, but sleeping instead")
	}

	select {
	case <-waker.Sleep():
		require.Fail(t, "unexpected wakeup")
	default:
	}
}
