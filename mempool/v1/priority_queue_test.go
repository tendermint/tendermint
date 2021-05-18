package v1

import (
	"math/rand"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTxPriorityQueue(t *testing.T) {
	pq := NewTxPriorityQueue()
	numTxs := 1000

	priorities := make([]int, numTxs)

	var wg sync.WaitGroup
	for i := 1; i <= numTxs; i++ {
		priorities[i-1] = i
		wg.Add(1)

		go func(i int) {
			pq.PushTx(&WrappedTx{
				Priority:  int64(i),
				Timestamp: time.Now(),
			})

			wg.Done()
		}(i)
	}

	sort.Sort(sort.Reverse(sort.IntSlice(priorities)))

	wg.Wait()
	require.Equal(t, numTxs, pq.NumTxs())

	// Wait a second and push a tx with a duplicate priority
	time.Sleep(time.Second)
	now := time.Now()
	pq.PushTx(&WrappedTx{
		Priority:  1000,
		Timestamp: now,
	})
	require.Equal(t, 1001, pq.NumTxs())

	tx := pq.PopTx()
	require.Equal(t, 1000, pq.NumTxs())
	require.Equal(t, int64(1000), tx.Priority)
	require.NotEqual(t, now, tx.Timestamp)

	gotPriorities := make([]int, 0)
	for pq.NumTxs() > 0 {
		gotPriorities = append(gotPriorities, int(pq.PopTx().Priority))
	}

	require.Equal(t, priorities, gotPriorities)
}

func TestTxPriorityQueue_GetEvictableTx(t *testing.T) {
	pq := NewTxPriorityQueue()
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	values := make([]int, 1000)

	for i := 0; i < 1000; i++ {
		x := rng.Intn(100000)
		pq.PushTx(&WrappedTx{
			Priority: int64(x),
		})

		values[i] = x
	}

	sort.Ints(values)

	max := values[len(values)-1]
	min := values[0]

	require.NotNil(t, pq.GetEvictableTx(int64(max+1)))
	require.NotNil(t, pq.GetEvictableTx(int64(min+1)))
	require.Nil(t, pq.GetEvictableTx(int64(min-1)))
	require.Nil(t, pq.GetEvictableTx(int64(min)))
}

func TestTxPriorityQueue_RemoveTx(t *testing.T) {
	pq := NewTxPriorityQueue()
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	numTxs := 1000

	values := make([]int, numTxs)

	for i := 0; i < numTxs; i++ {
		x := rng.Intn(100000)
		pq.PushTx(&WrappedTx{
			Priority: int64(x),
		})

		values[i] = x
	}

	require.Equal(t, numTxs, pq.NumTxs())

	sort.Ints(values)
	max := values[len(values)-1]

	wtx := pq.txs[pq.NumTxs()/2]
	pq.RemoveTx(wtx)
	require.Equal(t, numTxs-1, pq.NumTxs())
	require.Equal(t, int64(max), pq.PopTx().Priority)
}
