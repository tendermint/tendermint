package p2p

import (
	"math"
	"math/rand"
	"testing"
	"time"

	gogotypes "github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/log"
)

type testMessage = gogotypes.StringValue

func TestWDRRQueue_EqualWeights(t *testing.T) {
	chDescs := []ChannelDescriptor{
		{ID: 0x01, MaxSendBytes: 4, MaxSendCapacityBytes: 4 * 5},
		{ID: 0x02, MaxSendBytes: 4, MaxSendCapacityBytes: 4 * 5},
		{ID: 0x03, MaxSendBytes: 4, MaxSendCapacityBytes: 4 * 5},
		{ID: 0x04, MaxSendBytes: 4, MaxSendCapacityBytes: 4 * 5},
		{ID: 0x05, MaxSendBytes: 4, MaxSendCapacityBytes: 4 * 5},
		{ID: 0x06, MaxSendBytes: 4, MaxSendCapacityBytes: 4 * 5},
	}

	peerQueue := newWDRRQueue(log.NewNopLogger(), NodeID("FFAA"), NopMetrics(), chDescs)
	peerQueue.start()

	totalMsgs := make(map[ChannelID]int)
	deliveredMsgs := make(map[ChannelID]int)
	successRates := make(map[ChannelID]float64)

	doneCh := make(chan struct{})

	go func() {
		timout := 10 * time.Second
		ticker := time.NewTicker(timout)
		defer ticker.Stop()

		for {
			select {
			case e := <-peerQueue.dequeue():
				deliveredMsgs[e.channelID]++
				ticker.Reset(timout)

			case <-ticker.C:
				close(doneCh)
			}
		}
	}()

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	maxMsgs := 5000
	minMsgs := 1000

	for _, chDesc := range chDescs {
		total := rng.Intn(maxMsgs-minMsgs) + minMsgs // total = rand[minMsgs, maxMsgs)
		totalMsgs[ChannelID(chDesc.ID)] = total

		go func(cID ChannelID, n int) {
			for i := 0; i < n; i++ {
				peerQueue.enqueue() <- Envelope{
					channelID: cID,
					Message:   &testMessage{Value: "foo"},
				}
			}
		}(ChannelID(chDesc.ID), total)
	}

	// wait for dequeueing to complete
	<-doneCh

	// close queue and wait for cleanup
	peerQueue.close()
	<-peerQueue.closed()

	var (
		sum    float64
		stdDev float64
	)

	for chID, flow := range peerQueue.flows {
		require.Zero(t, flow.size, "expected size to be zero")
		require.Len(t, flow.buffer, 0, "expected buffer to be empty")

		total := totalMsgs[chID]
		delivered := deliveredMsgs[chID]
		successRate := float64(delivered) / float64(total)

		sum += successRate
		successRates[chID] = successRate

		// require some messages dropped
		require.Less(t, delivered, total, "expected some messages to be dropped")
		require.Less(t, successRate, 1.0, "expected a success rate below 100%")
	}

	numFlows := float64(len(peerQueue.flows))
	mean := sum / numFlows

	for _, successRate := range successRates {
		stdDev += math.Pow(successRate-mean, 2)
	}

	stdDev = math.Sqrt(stdDev / numFlows)
	require.Less(t, stdDev, 0.02, "expected success rate standard deviation to be less than 2%")
}
