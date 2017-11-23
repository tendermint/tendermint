package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHumanCode(t *testing.T) {
	assert.Equal(t, "Internal error", HumanCode(CodeType_InternalError))
	assert.Equal(t, "Unknown code", HumanCode(-1))
}
