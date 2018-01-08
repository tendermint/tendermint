package data_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	data "github.com/tendermint/go-wire/data"
)

// Key
type Key interface{}

// implementations
type Cool struct{ data.Bytes }
type Lame struct{ data.Bytes }

var keyMapper data.Mapper

// register both private key types with go-wire/data (and thus go-wire)
func init() {
	keyMapper = data.NewMapper(KeyS{}).
		RegisterImplementation(Cool{}, "cool", 1).
		RegisterImplementation(Lame{}, "lame", 88)
}

// KeyS adds json serialization to Key
type KeyS struct {
	Key
}

func (p KeyS) MarshalJSON() ([]byte, error) {
	return keyMapper.ToJSON(p.Key)
}

func (p *KeyS) UnmarshalJSON(data []byte) (err error) {
	parsed, err := keyMapper.FromJSON(data)
	if err == nil {
		p.Key = parsed.(Key)
	}
	return
}

func TestSimpleText(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	data.Encoder = data.HexEncoder
	cases := []struct {
		key      Key
		expected string
	}{
		{key: Cool{data.Bytes{0x42, 0x69}}, expected: "cool:4269"},
		{key: Lame{data.Bytes{0x70, 0xA3, 0x1e}}, expected: "lame:70A31E"},
	}

	for _, tc := range cases {
		wrap := KeyS{tc.key}
		// check json
		_, err := data.ToJSON(wrap)
		require.Nil(err, "%+v", err)
		// check text
		text, err := data.ToText(wrap)
		require.Nil(err, "%+v", err)
		assert.Equal(tc.expected, text)
	}
}

func TestBytesTest(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	data.Encoder = data.HexEncoder
	cases := []struct {
		bytes    data.Bytes
		expected string
	}{
		{data.Bytes{0x34, 0x54}, "3454"},
		{data.Bytes{}, ""},
		{data.Bytes{0xde, 0xad, 0xbe, 0x66}, "DEADBE66"},
	}

	for _, tc := range cases {
		text, err := data.ToText(tc.bytes)
		require.Nil(err, "%+v", err)
		assert.Equal(tc.expected, text)
	}
}
