package p2p

import (
	"math"
	"math/rand"
	"testing"
	"time"

	gogotypes "github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/require"
	tmsync "github.com/tendermint/tendermint/internal/libs/sync"
	"github.com/tendermint/tendermint/libs/log"
)

type testMessage = gogotypes.StringValue

func TestWDRRQueue_EqualWeights(t *testing.T) {
	chDescs := []ChannelDescriptor{
		{ID: 0x01, Priority: 1, MaxSendBytes: 4},
		{ID: 0x02, Priority: 1, MaxSendBytes: 4},
		{ID: 0x03, Priority: 1, MaxSendBytes: 4},
		{ID: 0x04, Priority: 1, MaxSendBytes: 4},
		{ID: 0x05, Priority: 1, MaxSendBytes: 4},
		{ID: 0x06, Priority: 1, MaxSendBytes: 4},
	}

	peerQueue := newWDRRScheduler(log.NewNopLogger(), NopMetrics(), chDescs, 1000, 1000, 120)
	peerQueue.start()

	totalMsgs := make(map[ChannelID]int)
	deliveredMsgs := make(map[ChannelID]int)
	successRates := make(map[ChannelID]float64)

	closer := tmsync.NewCloser()

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
				closer.Close()
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
	<-closer.Done()

	// close queue and wait for cleanup
	peerQueue.close()
	<-peerQueue.closed()

	var (
		sum    float64
		stdDev float64
	)

	for _, chDesc := range peerQueue.chDescs {
		chID := ChannelID(chDesc.ID)
		require.Zero(t, peerQueue.deficits[chID], "expected flow deficit to be zero")
		require.Len(t, peerQueue.buffer[chID], 0, "expected flow queue to be empty")

		total := totalMsgs[chID]
		delivered := deliveredMsgs[chID]
		successRate := float64(delivered) / float64(total)

		sum += successRate
		successRates[chID] = successRate

		// require some messages dropped
		require.Less(t, delivered, total, "expected some messages to be dropped")
		require.Less(t, successRate, 1.0, "expected a success rate below 100%")
	}

	require.Zero(t, peerQueue.size, "expected scheduler size to be zero")

	numFlows := float64(len(peerQueue.buffer))
	mean := sum / numFlows

	for _, successRate := range successRates {
		stdDev += math.Pow(successRate-mean, 2)
	}

	stdDev = math.Sqrt(stdDev / numFlows)
	require.Less(t, stdDev, 0.02, "expected success rate standard deviation to be less than 2%")
}

func TestWDRRQueue_DecreasingWeights(t *testing.T) {
	chDescs := []ChannelDescriptor{
		{ID: 0x01, Priority: 18, MaxSendBytes: 4},
		{ID: 0x02, Priority: 10, MaxSendBytes: 4},
		{ID: 0x03, Priority: 2, MaxSendBytes: 4},
		{ID: 0x04, Priority: 1, MaxSendBytes: 4},
		{ID: 0x05, Priority: 1, MaxSendBytes: 4},
		{ID: 0x06, Priority: 1, MaxSendBytes: 4},
	}

	peerQueue := newWDRRScheduler(log.NewNopLogger(), NopMetrics(), chDescs, 0, 0, 500)
	peerQueue.start()

	totalMsgs := make(map[ChannelID]int)
	deliveredMsgs := make(map[ChannelID]int)
	successRates := make(map[ChannelID]float64)

	for _, chDesc := range chDescs {
		total := 1000
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

	closer := tmsync.NewCloser()

	go func() {
		timout := 20 * time.Second
		ticker := time.NewTicker(timout)
		defer ticker.Stop()

		for {
			select {
			case e := <-peerQueue.dequeue():
				deliveredMsgs[e.channelID]++
				ticker.Reset(timout)

			case <-ticker.C:
				closer.Close()
			}
		}
	}()

	// wait for dequeueing to complete
	<-closer.Done()

	// close queue and wait for cleanup
	peerQueue.close()
	<-peerQueue.closed()

	for i, chDesc := range peerQueue.chDescs {
		chID := ChannelID(chDesc.ID)
		require.Zero(t, peerQueue.deficits[chID], "expected flow deficit to be zero")
		require.Len(t, peerQueue.buffer[chID], 0, "expected flow queue to be empty")

		total := totalMsgs[chID]
		delivered := deliveredMsgs[chID]
		successRate := float64(delivered) / float64(total)

		successRates[chID] = successRate

		// Require some messages dropped. Note, the top weighted flows may not have
		// any dropped if lower priority non-empty queues always exist.
		if i > 2 {
			require.Less(t, delivered, total, "expected some messages to be dropped")
			require.Less(t, successRate, 1.0, "expected a success rate below 100%")
		}
	}

	require.Zero(t, peerQueue.size, "expected scheduler size to be zero")

	// require channel 0x01 to have the highest success rate due to its weight
	ch01Rate := successRates[ChannelID(chDescs[0].ID)]
	for i := 1; i < len(chDescs); i++ {
		require.GreaterOrEqual(t, ch01Rate, successRates[ChannelID(chDescs[i].ID)])
	}

	// require channel 0x02 to have the 2nd highest success rate due to its weight
	ch02Rate := successRates[ChannelID(chDescs[1].ID)]
	for i := 2; i < len(chDescs); i++ {
		require.GreaterOrEqual(t, ch02Rate, successRates[ChannelID(chDescs[i].ID)])
	}

	// require channel 0x03 to have the 3rd highest success rate due to its weight
	ch03Rate := successRates[ChannelID(chDescs[2].ID)]
	for i := 3; i < len(chDescs); i++ {
		require.GreaterOrEqual(t, ch03Rate, successRates[ChannelID(chDescs[i].ID)])
	}
}
