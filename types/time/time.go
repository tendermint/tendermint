package time

import (
	"sort"
	"time"
)

// Now returns UTC time rounded since the zero time.
func Now() time.Time {
	return time.Now().Round(0).UTC()
}

type WeightedTime struct {
	Time   time.Time
	Weight int64
}

func NewWeightedTime(time time.Time, weight int64) *WeightedTime {
	return &WeightedTime{
		Time:   time,
		Weight: weight,
	}
}

// WeightedMedian computes weighted median time for a given array of WeightedTime and the total voting power.
func WeightedMedian(weightedTimes []*WeightedTime, totalVotingPower int64) (res time.Time) {
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
