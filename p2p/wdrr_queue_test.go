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
					Message:   &testMessage{Value: "foo"}, // 5 bytes
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
		require.Zero(t, flow.getSize(), "expected flow buffer size to be zero")
		require.Zero(t, flow.getDeficit(), "expected flow deficit to be zero")
		require.Len(t, flow.buffer, 0, "expected flow buffer to be empty")

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

func TestWDRRQueue_DecreasingWeights(t *testing.T) {
	chDescs := []ChannelDescriptor{
		{ID: 0x01, MaxSendBytes: 16, MaxSendCapacityBytes: 4 * 5},
		{ID: 0x02, MaxSendBytes: 12, MaxSendCapacityBytes: 4 * 5},
		{ID: 0x03, MaxSendBytes: 8, MaxSendCapacityBytes: 4 * 5},
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

	for _, chDesc := range chDescs {
		total := 5000
		totalMsgs[ChannelID(chDesc.ID)] = total

		go func(cID ChannelID, n int) {
			for i := 0; i < n; i++ {
				peerQueue.enqueue() <- Envelope{
					channelID: cID,
					Message:   &testMessage{Value: "foo"}, // 5 bytes
				}
			}
		}(ChannelID(chDesc.ID), total)
	}

	// wait for dequeueing to complete
	<-doneCh

	// close queue and wait for cleanup
	peerQueue.close()
	<-peerQueue.closed()

	for chID, flow := range peerQueue.flows {
		require.Zero(t, flow.getSize(), "expected flow buffer size to be zero")
		require.Zero(t, flow.getDeficit(), "expected flow deficit to be zero")
		require.Len(t, flow.buffer, 0, "expected flow buffer to be empty")

		total := totalMsgs[chID]
		delivered := deliveredMsgs[chID]
		successRate := float64(delivered) / float64(total)

		successRates[chID] = successRate

		// require some messages dropped
		require.Less(t, delivered, total, "expected some messages to be dropped")
		require.Less(t, successRate, 1.0, "expected a success rate below 100%")
	}

	// require channel 0x01 to have the highest success rate due to its weight
	ch01Rate := successRates[ChannelID(chDescs[0].ID)]
	for i := 1; i < len(chDescs); i++ {
		require.Greater(t, ch01Rate, successRates[ChannelID(chDescs[i].ID)])
	}

	// require channel 0x02 to have the 2nd highest success rate due to its weight
	ch02Rate := successRates[ChannelID(chDescs[1].ID)]
	for i := 2; i < len(chDescs); i++ {
		require.Greater(t, ch02Rate, successRates[ChannelID(chDescs[i].ID)])
	}

	// require channel 0x03 to have the 3rd highest success rate due to its weight
	ch03Rate := successRates[ChannelID(chDescs[2].ID)]
	for i := 3; i < len(chDescs); i++ {
		require.Greater(t, ch03Rate, successRates[ChannelID(chDescs[i].ID)])
	}
}
