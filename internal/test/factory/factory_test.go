package factory

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tendermint/tendermint/pkg/metadata"
)

func TestMakeHeader(t *testing.T) {
	_, err := MakeHeader(&metadata.Header{})
	assert.NoError(t, err)
}
