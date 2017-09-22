package common

import (
	"strings"
	"time"

	"github.com/pkg/errors"
)

// TimeLayout helps to parse a date string of the format YYYY-MM-DD
//   Intended to be used with the following function:
// 	 time.Parse(TimeLayout, date)
var TimeLayout = "2006-01-02" //this represents YYYY-MM-DD

// ParseDateRange parses a date range string of the format start:end
//   where the start and end date are of the format YYYY-MM-DD.
//   The parsed dates are time.Time and will return the zero time for
//   unbounded dates, ex:
//   unbounded start:	:2000-12-31
//	 unbounded end: 	2000-12-31:
func ParseDateRange(dateRange string) (startDate, endDate time.Time, err error) {
	dates := strings.Split(dateRange, ":")
	if len(dates) != 2 {
		err = errors.New("bad date range, must be in format date:date")
		return
	}
	parseDate := func(date string) (out time.Time, err error) {
		if len(date) == 0 {
			return
		}
		out, err = time.Parse(TimeLayout, date)
		return
	}
	startDate, err = parseDate(dates[0])
	if err != nil {
		return
	}
	endDate, err = parseDate(dates[1])
	if err != nil {
		return
	}
	return
}
