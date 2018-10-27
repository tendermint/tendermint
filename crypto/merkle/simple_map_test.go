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
		{[]string{"key1"}, []string{"value1"}, "b584132ccf49674da0402b80a5875105433ec6a3"},
		{[]string{"key1"}, []string{"value2"}, "fb6253798eb4227d40573e70c7dc2863a3cd4338"},
		// swap order with 2 keys
		{[]string{"key1", "key2"}, []string{"value1", "value2"}, "1dd58b3af346582bdf75fe6935bf259affb6c79c"},
		{[]string{"key2", "key1"}, []string{"value2", "value1"}, "1dd58b3af346582bdf75fe6935bf259affb6c79c"},
		// swap order with 3 keys
		{[]string{"key1", "key2", "key3"}, []string{"value1", "value2", "value3"}, "0addc50c612302c2deb247da91f4892ca9484ee7"},
		{[]string{"key1", "key3", "key2"}, []string{"value1", "value3", "value2"}, "0addc50c612302c2deb247da91f4892ca9484ee7"},
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
