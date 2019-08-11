package proxy

import (
	"github.com/pkg/errors"
)

type errNoData struct{}

func (e errNoData) Error() string {
	return "No data returned for query"
}

// IsErrNoData checks whether an error is due to a query returning empty data
func IsErrNoData(err error) bool {
	_, ok := errors.Cause(err).(errNoData)
	return ok
}

func ErrNoData() error {
	return errors.Wrap(errNoData{}, "")
}
