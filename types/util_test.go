package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKVPairInt(t *testing.T) {
	assert.Equal(t, KVPairInt("a", 1), &KVPair{Key: "a", ValueType: KVPair_INT, ValueInt: 1})
}

func TestKVPairString(t *testing.T) {
	assert.Equal(t, KVPairString("a", "b"), &KVPair{Key: "a", ValueType: KVPair_STRING, ValueString: "b"})
}
