package factory

import (
	"testing"

	"github.com/tendermint/tendermint/types"
)

func TestMakeHeader(t *testing.T) {
	MakeHeader(t, &types.Header{})
}
