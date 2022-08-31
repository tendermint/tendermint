package report

import (
	"math"
	"sync"
	"time"

	"github.com/tendermint/tendermint/test/loadtime/payload"
	"github.com/tendermint/tendermint/types"
)

type blockStore interface {
	Height() int64
	Base() int64
	LoadBlock(int64) *types.Block
}

type Report struct {
	Max, Min, Avg time.Duration
	StdDev        int64
	All           []time.Duration
	ErrorCount    int
}

func GenerateFromBlockStore(s blockStore) (Report, error) {
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
				l := p.Time.AsTime().Sub(b.bt)
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
		for i := s.Base(); i < s.Height(); i++ {
			b := s.LoadBlock(i)
			for _, tx := range b.Data.Txs {
				txc <- txData{tx: tx, bt: b.Time}
			}
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
		} else if pd.l < r.Min {
			r.Min = pd.l
		}
		// Using an int64 here makes an assumption about the scale and quantity of the data we are processing.
		// If all latencies were 2 seconds, we would need around 4 billion records to overflow this.
		// We are therefore assuming that the data does not exceed these bounds.
		sum += int64(pd.l)
	}
	r.Avg = time.Duration(sum / int64(len(r.All)))
	return r, nil
}
