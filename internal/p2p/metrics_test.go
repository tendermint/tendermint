package p2p

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tendermint/tendermint/proto/tendermint/consensus"
	"github.com/tendermint/tendermint/proto/tendermint/p2p"
)

func TestValueToMetricsLabel(t *testing.T) {
	m := NopMetrics()
	r := &p2p.PexResponse{}
	str := m.ValueToMetricLabel(r)
	assert.Equal(t, "p2p_PexResponse", str)

	// subsequent calls to the function should produce the same result
	str = m.ValueToMetricLabel(r)
	assert.Equal(t, "p2p_PexResponse", str)
}

func BenchmarkValueToMetricsLabel(b *testing.B) {
	numGoRoutines := 16
	msgTypes := []interface{}{
		&p2p.PexResponse{},
		&p2p.PexRequest{},
		&p2p.PexAddress{},
		&consensus.HasVote{},
		&consensus.NewRoundStep{},
		&consensus.NewValidBlock{},
		&consensus.VoteSetBits{},
		&consensus.VoteSetMaj23{},
		&consensus.Vote{},
		&consensus.BlockPart{},
		&consensus.ProposalPOL{},
	}
	b.Run("RW Mutex Version", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			m := NopMetrics()
			wg := &sync.WaitGroup{}
			b.StartTimer()
			for j := 0; j < numGoRoutines; j++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for k := 0; k < 100; k++ {
						m.ValueToMetricLabelRW(msgTypes[k%len(msgTypes)])
					}
				}()
			}
			wg.Wait()
		}
	})
	b.Run("Mutex Version", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			m := NopMetrics()
			wg := &sync.WaitGroup{}
			b.StartTimer()
			for j := 0; j < numGoRoutines; j++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for k := 0; k < 100; k++ {
						m.ValueToMetricLabelM(msgTypes[k%len(msgTypes)])
					}
				}()
			}
			wg.Wait()
		}
	})
}

func BenchmarkValueToMetricsLabelPrefilled(b *testing.B) {
	numGoRoutines := 16
	msgTypes := []interface{}{
		&p2p.PexResponse{},
		&p2p.PexRequest{},
		&p2p.PexAddress{},
		&consensus.HasVote{},
		&consensus.NewRoundStep{},
		&consensus.NewValidBlock{},
		&consensus.VoteSetBits{},
		&consensus.VoteSetMaj23{},
		&consensus.Vote{},
		&consensus.BlockPart{},
		&consensus.ProposalPOL{},
	}
	// create a metric set and put all of the types into the label map.
	m := NopMetrics()
	for _, t := range msgTypes {
		m.ValueToMetricLabel(t)
	}
	b.Run("RW Mutex Version", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			wg := &sync.WaitGroup{}
			b.StartTimer()
			for j := 0; j < numGoRoutines; j++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for k := 0; k < 100; k++ {
						m.ValueToMetricLabelRW(msgTypes[k%len(msgTypes)])
					}
				}()
			}
			wg.Wait()
		}
	})
	b.Run("Mutex Version", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			wg := &sync.WaitGroup{}
			b.StartTimer()
			for j := 0; j < numGoRoutines; j++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for k := 0; k < 100; k++ {
						m.ValueToMetricLabelM(msgTypes[k%len(msgTypes)])
					}
				}()
			}
			wg.Wait()
		}
	})
}
