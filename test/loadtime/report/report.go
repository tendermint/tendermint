package report

import (
	"math"
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
	r := Report{
		Max: 0,
		Min: math.MaxInt64,
	}
	var sum int64
	for i := s.Base(); i < s.Height(); i++ {
		b := s.LoadBlock(i)
		for _, tx := range b.Data.Txs {
			p, err := payload.FromBytes(tx)
			if err != nil {
				r.ErrorCount++
				continue
			}
			t := p.Time.AsTime().Sub(b.Time)
			r.All = append(r.All, t)
			if t > r.Max {
				r.Max = t
			} else if t < r.Min {
				r.Min = t
			}

			// Using an int64 here makes an assumption about the scale and quantity of the data we are processing.
			// If all latencies were 2 seconds, we would need around 4 billion records to overflow this.
			// We are therefore assuming that the data does not exceed these bounds.
			sum += int64(t)
		}
	}
	r.Avg = time.Duration(sum / int64(len(r.All)))
	return r, nil
}
