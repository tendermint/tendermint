package common

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// ParseDate parses a date string of the format YYYY-MM-DD
func ParseDate(date string) (t time.Time, err error) {

	//get the time of invoice
	str := strings.Split(date, "-")
	var ymd = []int{}
	for _, i := range str {
		j, err := strconv.Atoi(i)
		if err != nil {
			return t, err
		}
		ymd = append(ymd, j)
	}
	if len(ymd) != 3 {
		return t, fmt.Errorf("Bad date parsing, not 3 segments") //never stack trace
	}
	if ymd[1] < 1 || ymd[1] > 12 {
		return t, fmt.Errorf("Month not between 1 and 12") //never stack trace
	}
	if ymd[2] > 31 {
		return t, fmt.Errorf("Day over 31") //never stack trace
	}

	t = time.Date(ymd[0], time.Month(ymd[1]), ymd[2], 0, 0, 0, 0, time.UTC)

	return t, nil
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
