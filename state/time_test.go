package state

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tmtime "github.com/tendermint/tendermint/libs/time"
)

func TestWeightedMedian(t *testing.T) {
	m := make([]*weightedTime, 3)

	t1 := tmtime.Now()
	t2 := t1.Add(5 * time.Second)
	t3 := t1.Add(10 * time.Second)

	m[2] = newWeightedTime(t1, 33) // faulty processes
	m[0] = newWeightedTime(t2, 40) // correct processes
	m[1] = newWeightedTime(t3, 27) // correct processes
	totalVotingPower := int64(100)

	median := weightedMedian(m, totalVotingPower)
	assert.Equal(t, t2, median)
	// median always returns value between values of correct processes
	assert.Equal(t, true, (median.After(t1) || median.Equal(t1)) &&
		(median.Before(t3) || median.Equal(t3)))

	m[1] = newWeightedTime(t1, 40) // correct processes
	m[2] = newWeightedTime(t2, 27) // correct processes
	m[0] = newWeightedTime(t3, 33) // faulty processes
	totalVotingPower = int64(100)

	median = weightedMedian(m, totalVotingPower)
	assert.Equal(t, t2, median)
	// median always returns value between values of correct processes
	assert.Equal(t, true, (median.After(t1) || median.Equal(t1)) &&
		(median.Before(t2) || median.Equal(t2)))

	m = make([]*weightedTime, 8)
	t4 := t1.Add(15 * time.Second)
	t5 := t1.Add(60 * time.Second)

	m[3] = newWeightedTime(t1, 10) // correct processes
	m[1] = newWeightedTime(t2, 10) // correct processes
	m[5] = newWeightedTime(t2, 10) // correct processes
	m[4] = newWeightedTime(t3, 23) // faulty processes
	m[0] = newWeightedTime(t4, 20) // correct processes
	m[7] = newWeightedTime(t5, 10) // faulty processes
	totalVotingPower = int64(83)

	median = weightedMedian(m, totalVotingPower)
	assert.Equal(t, t3, median)
	// median always returns value between values of correct processes
	assert.Equal(t, true, (median.After(t1) || median.Equal(t1)) &&
		(median.Before(t4) || median.Equal(t4)))
}

func TestIsTimely(t *testing.T) {
	genesisTime, err := time.Parse(time.RFC3339, "2019-03-13T23:00:00Z")
	require.NoError(t, err)
	testCases := []struct {
		name         string
		blockTime    time.Time
		localTime    time.Time
		precision    time.Duration
		msgDelay     time.Duration
		expectTimely bool
	}{
		{
			// Checking that the following inequality evaluates to true:
			// 1 - 2 < 0 < 1 + 2 + 1
			name:         "basic timely",
			blockTime:    genesisTime,
			localTime:    genesisTime.Add(1 * time.Millisecond),
			precision:    time.Millisecond * 2,
			msgDelay:     time.Millisecond,
			expectTimely: true,
		},
		{
			// Checking that the following inequality evaluates to false:
			// 3 - 2 < 0 < 3 + 2 + 1
			name:         "local time too large",
			blockTime:    genesisTime,
			localTime:    genesisTime.Add(3 * time.Millisecond),
			precision:    time.Millisecond * 2,
			msgDelay:     time.Millisecond,
			expectTimely: false,
		},
		{
			// Checking that the following inequality evaluates to false:
			// 0 - 2 < 2 < 2 + 1
			name:         "block time too large",
			blockTime:    genesisTime.Add(4 * time.Millisecond),
			localTime:    genesisTime,
			precision:    time.Millisecond * 2,
			msgDelay:     time.Millisecond,
			expectTimely: false,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ti := IsTimely(testCase.blockTime, testCase.localTime, testCase.precision, testCase.msgDelay)
			assert.Equal(t, testCase.expectTimely, ti)
		})
	}
}
