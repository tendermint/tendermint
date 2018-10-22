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
		{[]string{"key1"}, []string{"value1"}, "fa9bc106ffd932d919bee935ceb6cf2b3dd72d8f"},
		{[]string{"key1"}, []string{"value2"}, "e00e7dcfe54e9fafef5111e813a587f01ba9c3e8"},
		// swap order with 2 keys
		{[]string{"key1", "key2"}, []string{"value1", "value2"}, "eff12d1c703a1022ab509287c0f196130123d786"},
		{[]string{"key2", "key1"}, []string{"value2", "value1"}, "eff12d1c703a1022ab509287c0f196130123d786"},
		// swap order with 3 keys
		{[]string{"key1", "key2", "key3"}, []string{"value1", "value2", "value3"}, "b2c62a277c08dbd2ad73ca53cd1d6bfdf5830d26"},
		{[]string{"key1", "key3", "key2"}, []string{"value1", "value3", "value2"}, "b2c62a277c08dbd2ad73ca53cd1d6bfdf5830d26"},
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
