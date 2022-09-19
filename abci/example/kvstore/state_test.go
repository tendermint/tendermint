package kvstore

import (
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/crypto"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
)

func TestStateMarshalUnmarshal(t *testing.T) {
	var (
		// key3 is some key with random bytes that is not printable
		key3 = []byte{0xa7, 0x2b, 0x5f, 0x4e, 0x21, 0x80, 0x99, 0xee, 0xc2, 0xc9, 0x3e, 0x48, 0x12, 0x3d, 0x2c, 0x46}
		// value3 is some value with random bytes that is not printable
		value3 = []byte{0xfd, 0x64, 0x87, 0xe8, 0x42, 0x8c, 0x7f, 0xb7, 0xa1, 0xbe, 0xb8, 0xef, 0x78, 0x29, 0x17, 0xf0}
	)

	state := NewKvState(dbm.NewMemDB(), 1)
	assert.NoError(t, state.Set([]byte("key1"), []byte("value1")))
	assert.NoError(t, state.Set([]byte("key2"), []byte("value2")))
	assert.NoError(t, state.Set(key3, value3))
	assert.NoError(t, state.UpdateAppHash(state, nil, nil))
	apphash := state.GetAppHash()

	encoded, err := json.MarshalIndent(state, "", "   ")
	require.NoError(t, err)
	assert.NotEmpty(t, encoded)
	t.Log(string(encoded))

	decoded := NewKvState(dbm.NewMemDB(), 1)
	err = json.Unmarshal(encoded, &decoded)
	require.NoError(t, err)

	v1, err := decoded.Get([]byte("key1"))
	require.NoError(t, err)
	assert.EqualValues(t, []byte("value1"), v1)

	v2, err := decoded.Get([]byte("key2"))
	require.NoError(t, err)
	assert.EqualValues(t, []byte("value2"), v2)

	v3, err := decoded.Get([]byte("key2"))
	require.NoError(t, err)
	assert.EqualValues(t, []byte("value2"), v3)

	assert.EqualValues(t, apphash, decoded.GetAppHash())
}

func TestStateUnmarshal(t *testing.T) {
	const initialHeight = 12345678
	zeroAppHash := make(tmbytes.HexBytes, crypto.DefaultAppHashSize)
	type keyVals struct {
		key   []byte
		value []byte
	}
	type testCase struct {
		name              string
		encoded           []byte
		expectHeight      int64
		expectAppHash     tmbytes.HexBytes
		expectKeyVals     []keyVals
		expectDecodeError bool
	}

	testCases := []testCase{
		{
			name:              "zero json",
			encoded:           []byte{},
			expectDecodeError: true,
		},
		{
			name:          "empty json",
			encoded:       []byte{'{', '}'},
			expectHeight:  initialHeight,
			expectAppHash: zeroAppHash,
		},
		{
			name:          "only height",
			encoded:       []byte("{\"height\":1234}"),
			expectHeight:  1234,
			expectAppHash: zeroAppHash,
		},
		{
			name: "full",
			encoded: []byte(`{
				"height": 6531,
				"app_hash": "1C9ECEC90E28D2461650418635878A5C91E49F47586ECF75F2B0CBB94E897112",
				"items": {
				   "key1": "value1",
				   "key2": "value2"
				}}`),
			expectHeight:  6531,
			expectAppHash: mustHexDecode("1C9ECEC90E28D2461650418635878A5C91E49F47586ECF75F2B0CBB94E897112"),
			expectKeyVals: []keyVals{
				{[]byte("key1"), []byte("value1")},
				{[]byte("key2"), []byte("value2")},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			decoded := NewKvState(dbm.NewMemDB(), initialHeight)
			err := json.Unmarshal(tc.encoded, &decoded)
			if tc.expectDecodeError {
				require.Error(t, err, "decode error expected")
			} else {
				require.NoError(t, err, "decode error not expected")
			}

			if tc.expectHeight != 0 {
				assert.EqualValues(t, tc.expectHeight, decoded.GetHeight())
			}
			if tc.expectAppHash != nil {
				assert.EqualValues(t, tc.expectAppHash, decoded.GetAppHash())
			}
			for _, item := range tc.expectKeyVals {
				value, err := decoded.Get(item.key)
				assert.NoError(t, err)
				assert.EqualValues(t, item.value, value)
			}
		})
	}

}

func mustHexDecode(s string) []byte {
	bz, err := hex.DecodeString(s)
	if err != nil {
		panic(err.Error())
	}
	return bz
}
