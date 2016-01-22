package types

import (
	"time"
)

type Stats struct {
	Blocks         int64     `json:"blocks"`
	TotalTime      int64     `json:"total_time"`
	LastCommitTime time.Time `json:"last_commit_time"`

	MinCommitTime  int64 `json:"min_commit_time"`
	MaxCommitTime  int64 `json:"max_commit_time"`
	MeanCommitTime int64 `json:"mean_commit_time"`
}

func NewStats() *Stats {
	return &Stats{
		LastCommitTime: time.Now(),
		MinCommitTime:  100000000,
		MaxCommitTime:  0,
	}
}

func (s *Stats) Update() {
	newCommitTime := time.Now()
	since := newCommitTime.Sub(s.LastCommitTime).Nanoseconds()

	var newS Stats

	newS.Blocks += 1
	newS.LastCommitTime = newCommitTime

	// update min and max
	if since < s.MinCommitTime {
		newS.MinCommitTime = since
	}
	if since > s.MaxCommitTime {
		newS.MaxCommitTime = since
	}

	// update total and average
	newS.TotalTime += since
	newS.MeanCommitTime = int64(float64(newS.TotalTime) / float64(newS.Blocks))
	*s = newS
}
