package report

import (
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
}

func GenerateFromBlockStore(s blockStore) (Report, error) {
	r := Report{}
	for i := s.Base(); i < s.Height(); i++ {
		b := s.LoadBlock(i)
		for _, tx := range b.Data.Txs {
			p, err := payload.FromBytes(tx)
			if err != nil {
				return Report{}, err
			}
			t := p.Time.AsTime().Sub(b.Time)
			r.All = append(r.All, t)
		}
	}
	return r, nil
}
