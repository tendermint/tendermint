package common

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var (
	date  = time.Date(2015, time.Month(12), 31, 0, 0, 0, 0, time.UTC)
	date2 = time.Date(2016, time.Month(12), 31, 0, 0, 0, 0, time.UTC)
	zero  time.Time
)

func TestParseDateRange(t *testing.T) {
	assert := assert.New(t)

	var testDates = []struct {
		dateStr string
		start   time.Time
		end     time.Time
		errNil  bool
	}{
		{"2015-12-31:2016-12-31", date, date2, true},
		{"2015-12-31:", date, zero, true},
		{":2016-12-31", zero, date2, true},
		{"2016-12-31", zero, zero, false},
		{"2016-31-12:", zero, zero, false},
		{":2016-31-12", zero, zero, false},
	}

	for _, test := range testDates {
		start, end, err := ParseDateRange(test.dateStr)
		if test.errNil {
			assert.Nil(err)
			testPtr := func(want, have time.Time) {
				assert.True(have.Equal(want))
			}
			testPtr(test.start, start)
			testPtr(test.end, end)
		} else {
			assert.NotNil(err)
		}
	}
}
