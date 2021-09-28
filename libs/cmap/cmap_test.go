package cmap

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIterateKeysWithValues(t *testing.T) {
	cmap := NewCMap()

	for i := 1; i <= 10; i++ {
		cmap.Set(fmt.Sprintf("key%d", i), fmt.Sprintf("value%d", i))
	}

	// Testing size
	assert.Equal(t, 10, cmap.Size())
	assert.Equal(t, 10, len(cmap.Keys()))
	assert.Equal(t, 10, len(cmap.Values()))

	// Iterating Keys, checking for matching Value
	for _, key := range cmap.Keys() {
		val := strings.ReplaceAll(key, "key", "value")
		assert.Equal(t, val, cmap.Get(key))
	}

	// Test if all keys are within []Keys()
	keys := cmap.Keys()
	for i := 1; i <= 10; i++ {
		assert.Contains(t, keys, fmt.Sprintf("key%d", i), "cmap.Keys() should contain key")
	}

	// Delete 1 Key
	cmap.Delete("key1")

	assert.NotEqual(
		t,
		len(keys),
		len(cmap.Keys()),
		"[]keys and []Keys() should not be equal, they are copies, one item was removed",
	)
}

func TestContains(t *testing.T) {
	cmap := NewCMap()

	cmap.Set("key1", "value1")

	// Test for known values
	assert.True(t, cmap.Has("key1"))
	assert.Equal(t, "value1", cmap.Get("key1"))

	// Test for unknown values
	assert.False(t, cmap.Has("key2"))
	assert.Nil(t, cmap.Get("key2"))
}

func BenchmarkCMapHas(b *testing.B) {
	m := NewCMap()
	for i := 0; i < 1000; i++ {
		m.Set(string(rune(i)), i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Has(string(rune(i)))
	}
}

func TestCMap_GetOrSet_Parallel(t *testing.T) {

	tests := []struct {
		name        string
		newValue    interface{}
		parallelism int
	}{
		{"test1", "a", 4},
		{"test2", "a", 40},
		{"test3", "a", 1},
	}

	//nolint:scopelint
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm := NewCMap()

			wg := sync.WaitGroup{}
			wg.Add(tt.parallelism)
			for i := 0; i < tt.parallelism; i++ {
				go func() {
					defer wg.Done()
					gotValue, _ := cm.GetOrSet(tt.name, tt.newValue)
					assert.EqualValues(t, tt.newValue, gotValue)
				}()
			}
			wg.Wait()
		})
	}
}

func TestCMap_GetOrSet_Exists(t *testing.T) {
	cm := NewCMap()

	gotValue, exists := cm.GetOrSet("key", 1000)
	assert.False(t, exists)
	assert.EqualValues(t, 1000, gotValue)

	gotValue, exists = cm.GetOrSet("key", 2000)
	assert.True(t, exists)
	assert.EqualValues(t, 1000, gotValue)
}
