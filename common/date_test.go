package common

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var (
	date  = time.Date(2015, time.Month(12), 31, 0, 0, 0, 0, time.UTC)
	date2 = time.Date(2016, time.Month(12), 31, 0, 0, 0, 0, time.UTC)
)

func TestParseDate(t *testing.T) {
	assert := assert.New(t)

	var testDates = []struct {
		dateStr string
		date    time.Time
		errNil  bool
	}{
		{"2015-12-31", date, true},
		{"2015-31-12", date, false},
		{"12-31-2015", date, false},
		{"31-12-2015", date, false},
	}

	for _, test := range testDates {
		parsed, err := ParseDate(test.dateStr)
		switch test.errNil {
		case true:
			assert.Nil(err)
			assert.True(parsed.Equal(test.date), "parsed: %v, want %v", parsed, test.date)
		case false:
			assert.NotNil(err, "parsed %v, expected err %v", parsed, err)
		}
	}
}

func TestParseDateRange(t *testing.T) {
	assert := assert.New(t)

	var testDates = []struct {
		dateStr string
		start   *time.Time
		end     *time.Time
		errNil  bool
	}{
		{"2015-12-31:2016-12-31", &date, &date2, true},
		{"2015-12-31:", &date, nil, true},
		{":2016-12-31", nil, &date2, true},
		{"2016-12-31", nil, nil, false},
		{"2016-31-12:", nil, nil, false},
		{":2016-31-12", nil, nil, false},
	}

	for _, test := range testDates {
		start, end, err := ParseDateRange(test.dateStr)
		switch test.errNil {
		case true:
			assert.Nil(err)
			testPtr := func(want, have *time.Time) {
				if want == nil {
					assert.Nil(have)
				} else {
					assert.True((*have).Equal(*want))
				}
			}
			testPtr(test.start, start)
			testPtr(test.end, end)
		case false:
			assert.NotNil(err)
		}
	}
}
