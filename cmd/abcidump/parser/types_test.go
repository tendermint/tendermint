package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewMessageType(t *testing.T) {
	types := map[string]bool{
		"tendermint.abci.Request":        true,
		"tendermint.abci.Response":       true,
		"tendermint.types.CoreChainLock": true,
		"tendermint.abci.RequestNX":      false,
	}
	for typeName, isCorrect := range types {
		t.Run(typeName, func(t *testing.T) {
			object, err := NewMessageType(typeName)
			if isCorrect {
				assert.NoError(t, err)
				assert.NotNil(t, object)
			} else {
				assert.Error(t, err)
			}
		})
	}
}
