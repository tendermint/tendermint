package common

import (
	"strings"
	"time"

	"github.com/pkg/errors"
)

// ParseDate parses a date string of the format YYYY-MM-DD
func ParseDate(date string) (t time.Time, err error) {
	layout := "2006-01-02" //this represents YYYY-MM-DD
	return time.Parse(layout, date)
}

// ParseDateRange parses a date range string of the format start:end
//   where the start and end date are of the format YYYY-MM-DD.
//   The parsed dates are *time.Time and will return nil pointers for
//   unbounded dates, ex:
//   unbounded start:	:2000-12-31
//	 unbounded end: 	2000-12-31:
func ParseDateRange(dateRange string) (startDate, endDate *time.Time, err error) {
	dates := strings.Split(dateRange, ":")
	if len(dates) != 2 {
		return nil, nil, errors.New("bad date range, must be in format date:date")
	}
	parseDate := func(date string) (*time.Time, error) {
		if len(date) == 0 {
			return nil, nil
		}
		d, err := ParseDate(date)
		return &d, err
	}
	startDate, err = parseDate(dates[0])
	if err != nil {
		return nil, nil, err
	}
	endDate, err = parseDate(dates[1])
	if err != nil {
		return nil, nil, err
	}
	return
}
