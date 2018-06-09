package proxy

import (
	"fmt"

	cmn "github.com/tendermint/tmlibs/common"
)

//--------------------------------------------

var errNoData = fmt.Errorf("No data returned for query")

// IsNoDataErr checks whether an error is due to a query returning empty data
func IsNoDataErr(err error) bool {
	return errors.Cause(err) == errNoData
}

func ErrNoData() error {
	return errors.WithStack(errNoData)
}

//--------------------------------------------
