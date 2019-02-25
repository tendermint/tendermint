package privval

import (
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	cmn "github.com/tendermint/tendermint/libs/common"
	"testing"
)

func TestIsConnTimeoutForNonTimeoutErrors(t *testing.T) {
	assert.False(t, IsConnTimeout(cmn.ErrorWrap(ErrDialRetryMax, "max retries exceeded")))
	assert.False(t, IsConnTimeout(errors.New("completely irrelevant error")))
}
