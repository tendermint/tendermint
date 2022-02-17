package state

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

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
