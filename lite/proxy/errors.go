package proxy

import (
	cmn "github.com/tendermint/tendermint/libs/common"
)

type errNoData struct{}

func (e errNoData) Error() string {
	return "No data returned for query"
}

// IsErrNoData checks whether an error is due to a query returning empty data
func IsErrNoData(err error) bool {
	if err_, ok := err.(cmn.Error); ok {
		_, ok := err_.Data().(errNoData)
		return ok
	}
	return false
}

func ErrNoData() error {
	return cmn.ErrorWrap(errNoData{}, "")
}
