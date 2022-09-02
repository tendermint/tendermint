package report

import (
	"math"
	"sync"
	"time"

	"github.com/tendermint/tendermint/test/loadtime/payload"
	"github.com/tendermint/tendermint/types"
	"gonum.org/v1/gonum/stat"
)

// BlockStore defines the set of methods needed by the report generator from
// Tendermint's store.Blockstore type. Using an interface allows for tests to
// more easily simulate the required behavior without having to use the more
// complex real API.
type BlockStore interface {
	Height() int64
	Base() int64
	LoadBlock(int64) *types.Block
}

// Report contains the data calculated from reading the timestamped transactions
// of each block found in the blockstore.
type Report struct {
	Max, Min, Avg, StdDev time.Duration

	// ErrorCount is the number of parsing errors encountered while reading the
	// transaction data. Parsing errors may occur if a transaction not generated
	// by the payload package is submitted to the chain.
	ErrorCount int

	// All contains all data points gathered from all valid transactions.
	All []time.Duration
}

// GenerateFromBlockStore creates a Report using the data in the provided
// BlockStore.
func GenerateFromBlockStore(s BlockStore) (Report, error) {
	type payloadData struct {
		l   time.Duration
		err error
	}
	type txData struct {
		tx []byte
		bt time.Time
	}

	const poolSize = 16

	txc := make(chan txData)
	pdc := make(chan payloadData, poolSize)

	wg := &sync.WaitGroup{}
	wg.Add(poolSize)
	for i := 0; i < poolSize; i++ {
		go func() {
			defer wg.Done()
			for b := range txc {
				p, err := payload.FromBytes(b.tx)
				if err != nil {
					pdc <- payloadData{err: err}
					continue
				}

				l := b.bt.Sub(p.Time.AsTime())
				pdc <- payloadData{l: l}
			}
		}()
	}
	go func() {
		wg.Wait()
		close(pdc)
	}()

	r := Report{
		Max: 0,
		Min: math.MaxInt64,
	}
	var sum int64
	go func() {
		base, height := s.Base(), s.Height()
		prev := s.LoadBlock(base)
		for i := base + 1; i < height; i++ {
			// Data from two adjacent block are used here simultaneously,
			// blocks of height H and H+1. The transactions of the block of
			// height H are used with the timestamp from the block of height
			// H+1. This is done because the timestamp from H+1 is calculated
			// by using the precommits submitted at height H. The timestamp in
			// block H+1 represents the time at which block H was committed.
			//
			// In the (very unlikely) event that the very last block of the
			// chain contains payload transactions, those transactions will not
			// be used in the latency calculations because the last block whose
			// transactions are used is the block one before the last.
			cur := s.LoadBlock(i)
			for _, tx := range prev.Data.Txs {
				txc <- txData{tx: tx, bt: cur.Time}
			}
			prev = cur
		}
		close(txc)
	}()
	for pd := range pdc {
		if pd.err != nil {
			r.ErrorCount++
			continue
		}
		r.All = append(r.All, pd.l)
		if pd.l > r.Max {
			r.Max = pd.l
		}
		if pd.l < r.Min {
			r.Min = pd.l
		}
		// Using an int64 here makes an assumption about the scale and quantity of the data we are processing.
		// If all latencies were 2 seconds, we would need around 4 billion records to overflow this.
		// We are therefore assuming that the data does not exceed these bounds.
		sum += int64(pd.l)
	}
	if len(r.All) == 0 {
		r.Min = 0
		return r, nil
	}
	r.Avg = time.Duration(sum / int64(len(r.All)))
	r.StdDev = time.Duration(int64(stat.StdDev(toFloat(r.All), nil)))
	return r, nil
}

func toFloat(in []time.Duration) []float64 {
	r := make([]float64, len(in))
	for i, v := range in {
		r[i] = float64(int64(v))
	}
	return r
}
