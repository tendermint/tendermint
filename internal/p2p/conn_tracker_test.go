package p2p

import (
	"math"
	"math/rand"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func randByte() byte {
	return byte(rand.Intn(math.MaxUint8))
}

func randLocalIPv4() net.IP {
	return net.IPv4(127, randByte(), randByte(), randByte())
}

func TestConnTracker(t *testing.T) {
	for name, factory := range map[string]func() connectionTracker{
		"BaseSmall": func() connectionTracker {
			return newConnTracker(10, time.Second)
		},
		"BaseLarge": func() connectionTracker {
			return newConnTracker(100, time.Hour)
		},
	} {
		t.Run(name, func(t *testing.T) {
			factory := factory // nolint:scopelint
			t.Run("Initialized", func(t *testing.T) {
				ct := factory()
				require.Equal(t, 0, ct.Len())
			})
			t.Run("RepeatedAdding", func(t *testing.T) {
				ct := factory()
				ip := randLocalIPv4()
				require.NoError(t, ct.AddConn(ip))
				for i := 0; i < 100; i++ {
					_ = ct.AddConn(ip)
				}
				require.Equal(t, 1, ct.Len())
			})
			t.Run("AddingMany", func(t *testing.T) {
				ct := factory()
				for i := 0; i < 100; i++ {
					_ = ct.AddConn(randLocalIPv4())
				}
				require.Equal(t, 100, ct.Len())
			})
			t.Run("Cycle", func(t *testing.T) {
				ct := factory()
				for i := 0; i < 100; i++ {
					ip := randLocalIPv4()
					require.NoError(t, ct.AddConn(ip))
					ct.RemoveConn(ip)
				}
				require.Equal(t, 0, ct.Len())
			})
		})
	}
	t.Run("VeryShort", func(t *testing.T) {
		ct := newConnTracker(10, time.Microsecond)
		for i := 0; i < 10; i++ {
			ip := randLocalIPv4()
			require.NoError(t, ct.AddConn(ip))
			time.Sleep(2 * time.Microsecond)
			require.NoError(t, ct.AddConn(ip))
		}
		require.Equal(t, 10, ct.Len())
	})
	t.Run("Window", func(t *testing.T) {
		const window = 100 * time.Millisecond
		ct := newConnTracker(10, window)
		ip := randLocalIPv4()
		require.NoError(t, ct.AddConn(ip))
		ct.RemoveConn(ip)
		require.Error(t, ct.AddConn(ip))
		time.Sleep(window)
		require.NoError(t, ct.AddConn(ip))
	})

}
