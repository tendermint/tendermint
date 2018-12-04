package merkle

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSimpleMap(t *testing.T) {
	tests := []struct {
		keys   []string
		values []string // each string gets converted to []byte in test
		want   string
	}{
		{[]string{"key1"}, []string{"value1"}, "321d150de16dceb51c72981b432b115045383259b1a550adf8dc80f927508967"},
		{[]string{"key1"}, []string{"value2"}, "2a9e4baf321eac99f6eecc3406603c14bc5e85bb7b80483cbfc75b3382d24a2f"},
		// swap order with 2 keys
		{[]string{"key1", "key2"}, []string{"value1", "value2"}, "c4d8913ab543ba26aa970646d4c99a150fd641298e3367cf68ca45fb45a49881"},
		{[]string{"key2", "key1"}, []string{"value2", "value1"}, "c4d8913ab543ba26aa970646d4c99a150fd641298e3367cf68ca45fb45a49881"},
		// swap order with 3 keys
		{[]string{"key1", "key2", "key3"}, []string{"value1", "value2", "value3"}, "b23cef00eda5af4548a213a43793f2752d8d9013b3f2b64bc0523a4791196268"},
		{[]string{"key1", "key3", "key2"}, []string{"value1", "value3", "value2"}, "b23cef00eda5af4548a213a43793f2752d8d9013b3f2b64bc0523a4791196268"},
	}
	for i, tc := range tests {
		db := newSimpleMap()
		for i := 0; i < len(tc.keys); i++ {
			db.Set(tc.keys[i], []byte(tc.values[i]))
		}
		got := db.Hash()
		assert.Equal(t, tc.want, fmt.Sprintf("%x", got), "Hash didn't match on tc %d", i)
	}
}
