package report_test

import (
	"testing"
	"time"

	"github.com/tendermint/tendermint/test/loadtime/payload"
	"github.com/tendermint/tendermint/test/loadtime/report"
	"github.com/tendermint/tendermint/types"
	"google.golang.org/protobuf/types/known/timestamppb"
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
	tn := time.Now()
	b1, err := payload.NewBytes(&payload.Payload{
		Time: timestamppb.New(tn.Add(-6 * time.Second)),
		Size: 1024,
	})
	if err != nil {
		t.Fatalf("generating payload %s", err)
	}
	b2, err := payload.NewBytes(&payload.Payload{
		Time: timestamppb.New(tn.Add(-4 * time.Second)),
		Size: 1024,
	})
	if err != nil {
		t.Fatalf("generating payload %s", err)
	}
	s := &mockBlockStore{
		blocks: []*types.Block{
			{
				Data: types.Data{
					Txs: []types.Tx{b1, b2},
				},
			},
			{
				// The timestamp from block H+1 is used to calculate the
				// latency for the transactions in block H.
				Header: types.Header{
					Time: tn,
				},
				Data: types.Data{
					Txs: []types.Tx{[]byte("error")},
				},
			},
			{
				Data: types.Data{
					Txs: []types.Tx{},
				},
			},
		},
	}
	r, err := report.GenerateFromBlockStore(s)
	if err != nil {
		t.Fatalf("generating report %s", err)
	}
	if len(r.All) != 2 {
		t.Fatalf("report contained different number of data points from expected. Expected %d but contained %d", 2, len(r.All))
	}
	if r.ErrorCount != 1 {
		t.Fatalf("ErrorCount did not match expected. Expected %d but contained %d", 1, r.ErrorCount)
	}
	if r.Avg != 5*time.Second {
		t.Fatalf("Avg did not match expected. Expected %s but contained %s", 5*time.Second, r.Avg)
	}
	if r.Min != 4*time.Second {
		t.Fatalf("Min did not match expected. Expected %s but contained %s", 4*time.Second, r.Min)
	}
	if r.Max != 6*time.Second {
		t.Fatalf("Max did not match expected. Expected %s but contained %s", 6*time.Second, r.Max)
	}
	// Verified using online standard deviation calculator:
	// https://www.calculator.net/standard-deviation-calculator.html?numberinputs=6%2C+4&ctype=p&x=84&y=27
	expectedStdDev := 1414213562 * time.Nanosecond
	if r.StdDev != expectedStdDev {
		t.Fatalf("StdDev did not match expected. Expected %s but contained %s", expectedStdDev, r.StdDev)
	}
}
