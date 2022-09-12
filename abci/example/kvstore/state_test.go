package kvstore

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func TestStateMarshalUnmarshal(t *testing.T) {
	state := NewKvState(dbm.NewMemDB())
	assert.NoError(t, state.Set([]byte("key1"), []byte("value1")))
	assert.NoError(t, state.Set([]byte("key2"), []byte("value2")))
	assert.NoError(t, state.UpdateAppHash(state, nil, nil))
	apphash := state.GetAppHash()

	encoded, err := json.MarshalIndent(state, "", "   ")
	require.NoError(t, err)
	assert.NotEmpty(t, encoded)

	decoded := NewKvState(dbm.NewMemDB())
	err = json.Unmarshal(encoded, &decoded)
	require.NoError(t, err)

	v1, err := decoded.Get([]byte("key1"))
	require.NoError(t, err)
	assert.EqualValues(t, []byte("value1"), v1)

	v2, err := decoded.Get([]byte("key2"))
	require.NoError(t, err)
	assert.EqualValues(t, []byte("value2"), v2)

	assert.EqualValues(t, apphash, decoded.GetAppHash())
}
