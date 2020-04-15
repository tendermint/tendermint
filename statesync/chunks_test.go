package statesync

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupChunkQueue(t *testing.T) (*chunkQueue, func()) {
	snapshot := &snapshot{
		Height:      3,
		Format:      1,
		ChunkHashes: [][]byte{},
		Metadata:    nil,
	}
	chunks := []*chunk{
		{Height: 3, Format: 1, Index: 0, Body: []byte{3, 1, 0}},
		{Height: 3, Format: 1, Index: 1, Body: []byte{3, 1, 1}},
		{Height: 3, Format: 1, Index: 2, Body: []byte{3, 1, 2}},
		{Height: 3, Format: 1, Index: 3, Body: []byte{3, 1, 3}},
		{Height: 3, Format: 1, Index: 4, Body: []byte{3, 1, 4}},
	}
	for _, c := range chunks {
		snapshot.ChunkHashes = append(snapshot.ChunkHashes, c.Hash())
	}
	queue, err := newChunkQueue(snapshot)
	require.NoError(t, err)
	teardown := func() {
		err := queue.Close()
		require.NoError(t, err)
	}
	return queue, teardown
}

func TestChunk_Hash(t *testing.T) {
	testcases := map[string]struct {
		chunk      *chunk
		expectHash string
	}{
		"nil chunk":    {nil, ""},
		"nil body":     {&chunk{Body: nil}, ""},
		"empty body":   {&chunk{Body: []byte{}}, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		"normal chunk": {&chunk{Body: []byte{1, 2, 3}}, "039058c6f2c0cb492c533b0a4d14ef77cc0f78abccced5287d84a1a2011cfb81"},
	}
	for name, tc := range testcases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.expectHash, hex.EncodeToString(tc.chunk.Hash()))
		})
	}
}

func TestChunkQueue(t *testing.T) {
	queue, teardown := setupChunkQueue(t)
	defer teardown()

	// Adding the first chunk should be fine
	added, err := queue.Add(&chunk{Height: 3, Format: 1, Index: 0, Body: []byte{3, 1, 0}})
	require.NoError(t, err)
	assert.True(t, added)

	// Adding the last chunk should also be fine
	added, err = queue.Add(&chunk{Height: 3, Format: 1, Index: 4, Body: []byte{3, 1, 4}})
	require.NoError(t, err)
	assert.True(t, added)

	// Adding the first or last chunks again should return false
	added, err = queue.Add(&chunk{Height: 3, Format: 1, Index: 0, Body: []byte{3, 1, 0}})
	require.NoError(t, err)
	assert.False(t, added)

	added, err = queue.Add(&chunk{Height: 3, Format: 1, Index: 4, Body: []byte{3, 1, 4}})
	require.NoError(t, err)
	assert.False(t, added)

	// Adding the remaining chunks in reverse should be fine
	added, err = queue.Add(&chunk{Height: 3, Format: 1, Index: 3, Body: []byte{3, 1, 3}})
	require.NoError(t, err)
	assert.True(t, added)

	added, err = queue.Add(&chunk{Height: 3, Format: 1, Index: 2, Body: []byte{3, 1, 2}})
	require.NoError(t, err)
	assert.True(t, added)

	added, err = queue.Add(&chunk{Height: 3, Format: 1, Index: 1, Body: []byte{3, 1, 1}})
	require.NoError(t, err)
	assert.True(t, added)

	// At this point, we should be able to retrieve them all via Next
	for i := 0; i < 5; i++ {
		c, err := queue.Next()
		require.NoError(t, err)
		assert.Equal(t, &chunk{Height: 3, Format: 1, Index: uint32(i), Body: []byte{3, 1, byte(i)}}, c)
	}
	_, err = queue.Next()
	require.Error(t, err)
	assert.Equal(t, errDone, err)

	// It should still be possible to try to add chunks (which will be ignored)
	added, err = queue.Add(&chunk{Height: 3, Format: 1, Index: 0, Body: []byte{3, 1, 0}})
	require.NoError(t, err)
	assert.False(t, added)

	// But after closing the queue, it should yield an error
	err = queue.Close()
	require.NoError(t, err)
	_, err = queue.Add(&chunk{Height: 3, Format: 1, Index: 0, Body: []byte{3, 1, 0}})
	require.Error(t, err)

	// Closing the queue again should also be fine
	err = queue.Close()
	require.NoError(t, err)
}

func TestChunkQueue_Add_ChunkErrors(t *testing.T) {
	testcases := map[string]struct {
		chunk *chunk
	}{
		"nil chunk":     {nil},
		"nil body":      {&chunk{Height: 3, Format: 1, Index: 0, Body: nil}},
		"wrong height":  {&chunk{Height: 9, Format: 1, Index: 0, Body: []byte{3, 1, 0}}},
		"wrong format":  {&chunk{Height: 3, Format: 9, Index: 0, Body: []byte{3, 1, 0}}},
		"invalid index": {&chunk{Height: 3, Format: 1, Index: 5, Body: []byte{3, 1, 0}}},
		"invalid body":  {&chunk{Height: 3, Format: 1, Index: 0, Body: []byte{3, 1, 1}}},
	}
	for name, tc := range testcases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			queue, teardown := setupChunkQueue(t)
			defer teardown()
			_, err := queue.Add(tc.chunk)
			require.Error(t, err)
		})
	}
}

func TestChunkQueue_Next(t *testing.T) {
	queue, teardown := setupChunkQueue(t)
	defer teardown()

	// Next should block waiting for the next chunks, even when given out of order.
	chNext := make(chan *chunk, 10)
	go func() {
		for {
			c, err := queue.Next()
			if err == errDone {
				close(chNext)
				break
			}
			require.NoError(t, err)
			chNext <- c
		}
	}()

	assert.Empty(t, chNext)
	_, err := queue.Add(&chunk{Height: 3, Format: 1, Index: 1, Body: []byte{3, 1, 1}})
	require.NoError(t, err)
	select {
	case <-chNext:
		assert.Fail(t, "channel should be empty")
	default:
	}

	_, err = queue.Add(&chunk{Height: 3, Format: 1, Index: 0, Body: []byte{3, 1, 0}})
	require.NoError(t, err)

	assert.Equal(t, &chunk{Height: 3, Format: 1, Index: 0, Body: []byte{3, 1, 0}}, <-chNext)
	assert.Equal(t, &chunk{Height: 3, Format: 1, Index: 1, Body: []byte{3, 1, 1}}, <-chNext)

	_, err = queue.Add(&chunk{Height: 3, Format: 1, Index: 4, Body: []byte{3, 1, 4}})
	require.NoError(t, err)
	select {
	case <-chNext:
		assert.Fail(t, "channel should be empty")
	default:
	}

	_, err = queue.Add(&chunk{Height: 3, Format: 1, Index: 2, Body: []byte{3, 1, 2}})
	require.NoError(t, err)
	_, err = queue.Add(&chunk{Height: 3, Format: 1, Index: 3, Body: []byte{3, 1, 3}})
	require.NoError(t, err)

	assert.Equal(t, &chunk{Height: 3, Format: 1, Index: 2, Body: []byte{3, 1, 2}}, <-chNext)
	assert.Equal(t, &chunk{Height: 3, Format: 1, Index: 3, Body: []byte{3, 1, 3}}, <-chNext)
	assert.Equal(t, &chunk{Height: 3, Format: 1, Index: 4, Body: []byte{3, 1, 4}}, <-chNext)

	_, ok := <-chNext
	assert.False(t, ok, "channel should be closed")

	// Calling next on a finished queue should return done
	_, err = queue.Next()
	assert.Equal(t, errDone, err)
}

func TestChunkQueue_Next_Closed(t *testing.T) {
	queue, teardown := setupChunkQueue(t)
	defer teardown()

	// Calling Next on a closed queue should return done
	_, err := queue.Add(&chunk{Height: 3, Format: 1, Index: 1, Body: []byte{3, 1, 1}})
	require.NoError(t, err)
	err = queue.Close()
	require.NoError(t, err)

	_, err = queue.Next()
	assert.Equal(t, errDone, err)
}

func TestChunkQueue_WaitFor(t *testing.T) {
	queue, teardown := setupChunkQueue(t)
	defer teardown()

	waitFor1 := queue.WaitFor(1)
	waitFor4 := queue.WaitFor(4)

	// Adding 0 and 2 should not trigger waiters
	_, err := queue.Add(&chunk{Height: 3, Format: 1, Index: 0, Body: []byte{3, 1, 0}})
	require.NoError(t, err)
	_, err = queue.Add(&chunk{Height: 3, Format: 1, Index: 2, Body: []byte{3, 1, 2}})
	require.NoError(t, err)
	select {
	case <-waitFor1:
		require.Fail(t, "WaitFor(1) should not trigger on 0 or 2")
	case <-waitFor4:
		require.Fail(t, "WaitFor(4) should not trigger on 0 or 2")
	default:
	}

	// Adding 1 should trigger WaitFor(1), but not WaitFor(4). The channel should be closed.
	_, err = queue.Add(&chunk{Height: 3, Format: 1, Index: 1, Body: []byte{3, 1, 1}})
	require.NoError(t, err)
	assert.True(t, <-waitFor1)
	_, ok := <-waitFor1
	assert.False(t, ok)
	select {
	case <-waitFor4:
		require.Fail(t, "WaitFor(4) should not trigger on 0 or 2")
	default:
	}

	// Fetch the first chunk. At this point, waiting for either 0 (retrieved from pool) or 1
	// (queued in pool) should immediately return true.
	c, err := queue.Next()
	require.NoError(t, err)
	assert.EqualValues(t, 0, c.Index)

	w := queue.WaitFor(0)
	assert.True(t, <-w)
	_, ok = <-w
	assert.False(t, ok)

	w = queue.WaitFor(1)
	assert.True(t, <-w)
	_, ok = <-w
	assert.False(t, ok)

	// Close the queue. This should cause the waiter for 4 to return false and close it,
	// and also cause any future waiters to immediately return false.
	err = queue.Close()
	require.NoError(t, err)
	assert.False(t, <-waitFor4)
	_, ok = <-waitFor4
	assert.False(t, ok)

	w = queue.WaitFor(3)
	assert.False(t, <-w)
	_, ok = <-w
	assert.False(t, ok)
}
