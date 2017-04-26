package rpcclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type Tx []byte

type Foo struct {
	Bar int
	Baz string
}

func TestArgToJSON(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	cases := []struct {
		input    interface{}
		expected string
	}{
		{[]byte("1234"), "0x31323334"},
		{Tx("654"), "0x363534"},
		{Foo{7, "hello"}, `{"Bar":7,"Baz":"hello"}`},
	}

	for i, tc := range cases {
		args := map[string]interface{}{"data": tc.input}
		err := argsToJson(args)
		require.Nil(err, "%d: %+v", i, err)
		require.Equal(1, len(args), "%d", i)
		data, ok := args["data"].(string)
		require.True(ok, "%d: %#v", i, args["data"])
		assert.Equal(tc.expected, data, "%d", i)
	}
}
