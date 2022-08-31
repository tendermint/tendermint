package report_test

import (
	"testing"
	"time"

	"github.com/tendermint/tendermint/test/loadtime/payload"
	"github.com/tendermint/tendermint/test/loadtime/report"
	"github.com/tendermint/tendermint/types"
)

type mockBlockStore struct {
	base   int64
	blocks []*types.Block
}

func (m *mockBlockStore) Height() int64 {
	return m.base + int64(len(m.blocks))
}

func (m *mockBlockStore) Base() int64 {
	return m.base
}

func (m *mockBlockStore) LoadBlock(i int64) *types.Block {
	return m.blocks[i-m.base]
}

func TestGenerateReport(t *testing.T) {
	b1, err := payload.NewBytes(payload.Options{
		Size: 1024,
	})
	if err != nil {
		t.Fatalf("generating payload %s", err)
	}
	b2, err := payload.NewBytes(payload.Options{
		Size: 1024,
	})
	if err != nil {
		t.Fatalf("generating payload %s", err)
	}
	s := &mockBlockStore{
		blocks: []*types.Block{
			{
				Header: types.Header{
					Time: time.Now(),
				},
				Data: types.Data{
					Txs: []types.Tx{b1, b2},
				},
			},
		},
	}
	r, err := report.GenerateFromBlockStore(s)
	if err != nil {
		t.Fatalf("generating report %s", err)
	}
	if len(r.All) != 2 {
		t.Fatalf("report contained different number of data points from expected. Expected %d but contained %d", 1, len(r.All))
	}
}
