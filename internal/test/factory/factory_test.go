package factory

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tendermint/tendermint/types"
)

func TestMakeHeader(t *testing.T) {
	_, err := MakeHeader(&types.Header{})
	assert.NoError(t, err)
}

func TestRandomNodeID(t *testing.T) {
	assert.NotPanics(t, func() { RandomNodeID() })
}
