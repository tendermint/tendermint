package factory

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tendermint/tendermint/pkg/meta"
)

func TestMakeHeader(t *testing.T) {
	_, err := MakeHeader(&meta.Header{})
	assert.NoError(t, err)
}
