package state

import (
	"sort"
	"time"
)

// weightedTime for computing a median.
type weightedTime struct {
	Time   time.Time
	Weight int64
}

// newWeightedTime with time and weight.
func newWeightedTime(time time.Time, weight int64) *weightedTime {
	return &weightedTime{
		Time:   time,
		Weight: weight,
	}
}

// weightedMedian computes weighted median time for a given array of WeightedTime and the total voting power.
func weightedMedian(weightedTimes []*weightedTime, totalVotingPower int64) (res time.Time) {
	median := totalVotingPower / 2

	sort.Slice(weightedTimes, func(i, j int) bool {
		if weightedTimes[i] == nil {
			return false
		}
		if weightedTimes[j] == nil {
			return true
		}
		return weightedTimes[i].Time.UnixNano() < weightedTimes[j].Time.UnixNano()
	})

	for _, weightedTime := range weightedTimes {
		if weightedTime != nil {
			if median <= weightedTime.Weight {
				res = weightedTime.Time
				break
			}
			median -= weightedTime.Weight
		}
	}
	return
}
